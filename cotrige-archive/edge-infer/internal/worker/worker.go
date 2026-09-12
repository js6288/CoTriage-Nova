package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/pangjian-pj/cotriage-dev/edge-infer/internal/config"
	"github.com/pangjian-pj/cotriage-dev/edge-infer/internal/event"
	"github.com/pangjian-pj/cotriage-dev/edge-infer/internal/infer"
	"github.com/redis/go-redis/v9"
)

type Worker struct {
	redis  *redis.Client
	cfg    config.Config
	engine *infer.Engine
}

func New(rdb *redis.Client, cfg config.Config, engine *infer.Engine) *Worker {
	return &Worker{redis: rdb, cfg: cfg, engine: engine}
}

func (worker *Worker) Run(ctx context.Context) error {
	claimTicker := time.NewTicker(worker.cfg.ClaimIdleTime / 2)
	defer claimTicker.Stop()
	cleanupTicker := time.NewTicker(worker.cfg.RedisCleanupEvery)
	defer cleanupTicker.Stop()

	for {
		if err := worker.ensureRedisReady(ctx); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-claimTicker.C:
			if err := worker.claimPending(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("[edge-infer] claim pending: %v", err)
			}
		case <-cleanupTicker.C:
			if err := worker.cleanupRedis(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("[edge-infer] redis cleanup: %v", err)
			}
		default:
		}

		streams, err := worker.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group: worker.cfg.ConsumerGroup, Consumer: worker.cfg.ConsumerName,
			Streams: []string{worker.cfg.RawStream, ">"}, Count: 16,
			Block: worker.cfg.BlockTimeout,
		}).Result()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Printf("[edge-infer] read group: %v; retry in %s", err, worker.cfg.RedisRetryDelay)
			if err := worker.sleep(ctx, worker.cfg.RedisRetryDelay); err != nil {
				return err
			}
			continue
		}
		for _, stream := range streams {
			for _, message := range stream.Messages {
				worker.handle(ctx, message)
			}
		}
	}
}

func (worker *Worker) ensureRedisReady(ctx context.Context) error {
	for {
		if err := worker.redis.Ping(ctx).Err(); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Printf("[edge-infer] redis unavailable: %v; retry in %s", err, worker.cfg.RedisRetryDelay)
			if err := worker.sleep(ctx, worker.cfg.RedisRetryDelay); err != nil {
				return err
			}
			continue
		}
		if err := worker.ensureGroup(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Printf("[edge-infer] ensure consumer group: %v; retry in %s", err, worker.cfg.RedisRetryDelay)
			if err := worker.sleep(ctx, worker.cfg.RedisRetryDelay); err != nil {
				return err
			}
			continue
		}
		return nil
	}
}

func (worker *Worker) ensureGroup(ctx context.Context) error {
	err := worker.redis.XGroupCreateMkStream(ctx, worker.cfg.RawStream, worker.cfg.ConsumerGroup, "0").Err()
	if err != nil && !isBusyGroup(err) {
		return fmt.Errorf("create consumer group: %w", err)
	}
	return nil
}

func (worker *Worker) cleanupRedis(ctx context.Context) error {
	streams := []struct {
		name   string
		maxLen int64
	}{
		{worker.cfg.RawStream, worker.cfg.RawMaxLen},
		{worker.cfg.InferredStream, worker.cfg.InferredMaxLen},
		{worker.cfg.DLQStream, worker.cfg.DLQMaxLen},
	}
	for _, stream := range streams {
		if err := worker.redis.Do(ctx, "XTRIM", stream.name, "MAXLEN", "~", stream.maxLen).Err(); err != nil {
			return fmt.Errorf("trim stream %s: %w", stream.name, err)
		}
	}
	_ = worker.redis.Expire(ctx, worker.attemptKey(), 24*time.Hour).Err()
	return nil
}

func (worker *Worker) claimPending(ctx context.Context) error {
	messages, _, err := worker.redis.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream: worker.cfg.RawStream, Group: worker.cfg.ConsumerGroup,
		Consumer: worker.cfg.ConsumerName, MinIdle: worker.cfg.ClaimIdleTime,
		Start: "0-0", Count: 16,
	}).Result()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		return err
	}
	for _, message := range messages {
		worker.handle(ctx, message)
	}
	return nil
}

func (worker *Worker) handle(ctx context.Context, message redis.XMessage) {
	if err := worker.process(ctx, message); err != nil {
		attempt, attemptErr := worker.redis.HIncrBy(ctx, worker.attemptKey(), message.ID, 1).Result()
		if attemptErr != nil {
			log.Printf("[edge-infer] message=%s error=%v; update attempt: %v", message.ID, err, attemptErr)
			return
		}
		_ = worker.redis.Expire(ctx, worker.attemptKey(), 24*time.Hour).Err()
		if attempt < worker.cfg.MaxAttempts {
			log.Printf("[edge-infer] message=%s attempt=%d/%d error=%v", message.ID, attempt, worker.cfg.MaxAttempts, err)
			return
		}
		if dlqErr := worker.moveToDLQ(ctx, message, attempt, err); dlqErr != nil {
			log.Printf("[edge-infer] message=%s move to DLQ: %v", message.ID, dlqErr)
			return
		}
		if ackErr := worker.ack(ctx, message.ID); ackErr != nil {
			log.Printf("[edge-infer] message=%s ACK after DLQ: %v", message.ID, ackErr)
		}
		return
	}
	if err := worker.ack(ctx, message.ID); err != nil {
		log.Printf("[edge-infer] message=%s ACK: %v", message.ID, err)
	}
}

func (worker *Worker) process(ctx context.Context, message redis.XMessage) error {
	streamEventID, ok := stringValue(message.Values["event_id"])
	if !ok || streamEventID == "" {
		return fmt.Errorf("stream event_id is missing")
	}
	payload, ok := stringValue(message.Values["payload"])
	if !ok || payload == "" {
		return fmt.Errorf("payload is missing")
	}

	var raw event.RawEvent
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	if raw.EventID != streamEventID {
		return fmt.Errorf("event_id mismatch: stream=%q payload=%q", streamEventID, raw.EventID)
	}

	processed, err := worker.redis.Exists(ctx, worker.processedKey(raw.EventID)).Result()
	if err != nil {
		return fmt.Errorf("check idempotency key: %w", err)
	}
	if processed > 0 {
		return nil
	}

	result, err := worker.engine.Infer(ctx, raw)
	if err != nil {
		return fmt.Errorf("infer event: %w", err)
	}
	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode inferred event: %w", err)
	}
	if err := worker.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: worker.cfg.InferredStream, MaxLen: worker.cfg.InferredMaxLen, Approx: true,
		Values: map[string]any{"event_id": raw.EventID, "payload": string(body)},
	}).Err(); err != nil {
		return fmt.Errorf("publish inferred event: %w", err)
	}
	if err := worker.redis.Set(ctx, worker.processedKey(raw.EventID), "1", 24*time.Hour).Err(); err != nil {
		return fmt.Errorf("write idempotency key: %w", err)
	}
	return nil
}

func (worker *Worker) moveToDLQ(ctx context.Context, message redis.XMessage, attempt int64, cause error) error {
	body, err := json.Marshal(message.Values)
	if err != nil {
		return err
	}
	return worker.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: worker.cfg.DLQStream, MaxLen: worker.cfg.DLQMaxLen, Approx: true,
		Values: map[string]any{
			"source_stream": worker.cfg.RawStream, "source_message_id": message.ID,
			"consumer_group": worker.cfg.ConsumerGroup, "attempt": attempt,
			"error": cause.Error(), "original_values": string(body),
			"failed_at": time.Now().UTC().Format(time.RFC3339Nano),
		},
	}).Err()
}

func (worker *Worker) ack(ctx context.Context, messageID string) error {
	pipe := worker.redis.TxPipeline()
	pipe.XAck(ctx, worker.cfg.RawStream, worker.cfg.ConsumerGroup, messageID)
	pipe.HDel(ctx, worker.attemptKey(), messageID)
	_, err := pipe.Exec(ctx)
	return err
}

func (worker *Worker) attemptKey() string {
	return "cotriage:edge-infer:attempts:" + worker.cfg.ConsumerGroup
}

func (worker *Worker) processedKey(eventID string) string {
	return "cotriage:edge-infer:processed:" + eventID
}

func (worker *Worker) sleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func stringValue(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		return typed, true
	case []byte:
		return string(typed), true
	default:
		return "", false
	}
}

func isBusyGroup(err error) bool {
	return err != nil && len(err.Error()) >= len("BUSYGROUP") && err.Error()[:len("BUSYGROUP")] == "BUSYGROUP"
}
