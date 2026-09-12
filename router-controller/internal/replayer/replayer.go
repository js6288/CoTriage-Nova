package replayer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/config"
	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/store"
	"github.com/redis/go-redis/v9"
)

type Publisher interface {
	PublishCloudRequest(ctx context.Context, item store.CloudOutboxModel) error
}

type RedisPublisher struct {
	Client *redis.Client
	Stream string
}

func (p RedisPublisher) PublishCloudRequest(ctx context.Context, item store.CloudOutboxModel) error {
	if p.Client == nil {
		return errors.New("redis client unavailable")
	}
	stream := p.Stream
	if stream == "" {
		stream = "cotriage.events.cloud.req"
	}
	values := map[string]any{
		"event_id":            item.EventID,
		"request_id":          item.RequestID,
		"idempotency_key":     item.IdempotencyKey,
		"payload_json":        item.PayloadJSON,
		"route_decision_json": item.RouteDecisionJSON,
		"priority_score":      item.PriorityScore,
		"source":              "router_outbox",
	}
	if err := p.Client.XAdd(ctx, &redis.XAddArgs{Stream: stream, Values: values}).Err(); err != nil {
		return fmt.Errorf("publish cloud request: %w", err)
	}
	return nil
}

type Replayer struct {
	Store          *store.Store
	Publisher      Publisher
	Worker         string
	Interval       time.Duration
	BatchSize      int
	LockTTL        time.Duration
	RetryBaseDelay time.Duration
	MaxAttempts    int
	Now            func() time.Time
	Logger         *log.Logger
}

func New(cfg config.Config, st *store.Store, publisher Publisher) *Replayer {
	interval := cfg.OutboxReplayerInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	batch := cfg.OutboxReplayerBatch
	if batch <= 0 {
		batch = 20
	}
	lockTTL := cfg.OutboxLockTTL
	if lockTTL <= 0 {
		lockTTL = 30 * time.Second
	}
	retryBase := cfg.OutboxRetryBaseDelay
	if retryBase <= 0 {
		retryBase = 5 * time.Second
	}
	maxAttempts := cfg.OutboxMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	return &Replayer{
		Store:          st,
		Publisher:      publisher,
		Worker:         cfg.ConsumerName,
		Interval:       interval,
		BatchSize:      batch,
		LockTTL:        lockTTL,
		RetryBaseDelay: retryBase,
		MaxAttempts:    maxAttempts,
		Now:            func() time.Time { return time.Now().UTC() },
	}
}

func (r *Replayer) Run(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	ticker := time.NewTicker(r.Interval)
	defer ticker.Stop()
	r.logf("outbox replayer started interval=%s batch=%d worker=%s", r.Interval, r.BatchSize, r.Worker)
	for {
		if err := r.ReplayOnce(ctx); err != nil {
			r.logf("outbox replay tick failed: %v", err)
		}
		select {
		case <-ctx.Done():
			r.logf("outbox replayer stopped")
			return
		case <-ticker.C:
		}
	}
}

func (r *Replayer) ReplayOnce(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if r.Store == nil {
		return errors.New("store unavailable")
	}
	if r.Publisher == nil {
		return errors.New("publisher unavailable")
	}
	now := r.now()
	if _, err := r.Store.RequeueExpiredSending(ctx, now, "outbox sending lock expired"); err != nil {
		return err
	}
	items, err := r.Store.ListDueOutbox(ctx, r.BatchSize, now)
	if err != nil {
		return err
	}
	var firstErr error
	for _, candidate := range items {
		locked, ok, err := r.Store.TryLockOutbox(ctx, candidate.ID, r.worker(), now, now.Add(r.LockTTL))
		if err != nil {
			firstErr = keepFirst(firstErr, err)
			continue
		}
		if !ok {
			continue
		}
		if locked.MaxAttempts <= 0 {
			locked.MaxAttempts = r.MaxAttempts
		}
		if err := r.Publisher.PublishCloudRequest(ctx, locked); err != nil {
			firstErr = keepFirst(firstErr, r.markFailure(ctx, locked, err))
			continue
		}
		if err := r.Store.MarkOutboxSent(ctx, locked.ID, r.now()); err != nil {
			firstErr = keepFirst(firstErr, err)
		}
	}
	return firstErr
}

func (r *Replayer) markFailure(ctx context.Context, item store.CloudOutboxModel, cause error) error {
	now := r.now()
	reason := cause.Error()
	maxAttempts := item.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = r.MaxAttempts
	}
	if item.Attempt >= maxAttempts {
		return r.Store.MarkOutboxDLQ(ctx, item.ID, reason, now)
	}
	nextRetry := now.Add(r.BackoffDelay(item.Attempt))
	return r.Store.MarkOutboxRetry(ctx, item.ID, nextRetry, reason, now)
}

func (r *Replayer) BackoffDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := r.RetryBaseDelay
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay > time.Minute {
			return time.Minute
		}
	}
	return delay
}

func (r *Replayer) now() time.Time {
	if r.Now == nil {
		return time.Now().UTC()
	}
	return r.Now().UTC()
}

func (r *Replayer) worker() string {
	if r.Worker == "" {
		return "router-controller"
	}
	return r.Worker
}

func (r *Replayer) logf(format string, args ...any) {
	if r.Logger != nil {
		r.Logger.Printf(format, args...)
		return
	}
	log.Printf("[router-controller] "+format, args...)
}

func keepFirst(first, next error) error {
	if first != nil {
		return first
	}
	return next
}
