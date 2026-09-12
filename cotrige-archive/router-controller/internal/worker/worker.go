package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/config"
	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/event"
	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/raise"
	routerRuntime "github.com/pangjian-pj/cotriage-dev/router-controller/internal/runtime"
	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/state"
	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/store"
	"github.com/redis/go-redis/v9"
)

type Worker struct {
	Config  config.Config
	Redis   *redis.Client
	Store   *store.Store
	Engine  raise.Router
	State   *state.Manager
	Runtime *routerRuntime.Builder
	Policy  raise.Policy
	Logger  *log.Logger
}

type RoutedEvent struct {
	Input         event.InferredEvent `json:"input"`
	Decision      raise.Decision      `json:"decision"`
	SystemState   raise.SystemState   `json:"system_state"`
	RoutedAt      string              `json:"routed_at"`
	Router        string              `json:"router"`
	SchemaVersion string              `json:"schema_version"`
}

func New(cfg config.Config, rdb *redis.Client, st *store.Store, engine raise.Router, manager *state.Manager, runtimeBuilder *routerRuntime.Builder) *Worker {
	if manager == nil {
		manager = state.NewManager(cfg.StateWindow, 7, cfg.GainUCBAlpha)
	}
	return &Worker{
		Config: cfg, Redis: rdb, Store: st, Engine: engine, State: manager, Runtime: runtimeBuilder,
	}
}

func (w *Worker) Run(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := w.ensureConsumerGroup(ctx); err != nil {
		w.logf("ensure consumer group failed: %v", err)
	}
	w.logf("main worker started stream=%s group=%s consumer=%s batch=%d", w.Config.InferredStream, w.Config.ConsumerGroup, w.Config.ConsumerName, w.Config.WorkerBatch)
	for {
		if err := w.ReadOnce(ctx); err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				w.logf("main worker stopped")
				return
			}
			w.logf("main worker tick failed: %v", err)
		}
		if ctx.Err() != nil {
			w.logf("main worker stopped")
			return
		}
	}
}

func (w *Worker) ReadOnce(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if w.Redis == nil || w.Store == nil || w.Engine == nil || w.State == nil || w.Runtime == nil {
		return errors.New("worker dependency unavailable")
	}
	streams, err := w.Redis.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: w.Config.ConsumerGroup, Consumer: w.Config.ConsumerName,
		Streams: []string{w.Config.InferredStream, ">"},
		Count:   int64(w.batch()), Block: w.Config.WorkerBlockTimeout,
	}).Result()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		return err
	}
	var firstErr error
	for _, stream := range streams {
		for _, message := range stream.Messages {
			if err := w.HandleMessage(ctx, message); err != nil {
				firstErr = keepFirst(firstErr, err)
				w.logf("handle inferred message id=%s failed: %v", message.ID, err)
			}
		}
	}
	return firstErr
}

func (w *Worker) HandleMessage(ctx context.Context, message redis.XMessage) error {
	now := time.Now().UTC()
	in, rawPayload, err := decodeInferred(message)
	if err != nil {
		return w.publishDLQAndAck(ctx, message, rawPayload, err)
	}
	if in.EventID == "" {
		in.EventID = "redis-" + message.ID
	}
	stats := w.State.BuildStats(in, now)
	systemState, stateErr := w.Runtime.Build(ctx, w.State.RecentCloudRatio(), 0, now)
	if stateErr != nil {
		w.logf("build runtime state degraded: %v", stateErr)
	}
	decision := w.Engine.Decide(in, systemState, stats, w.Policy)
	if decision.Action == raise.ActionCloud {
		w.Runtime.ConsumeCloudBudget(1, now)
	}
	routed := RoutedEvent{
		Input: in, Decision: decision, SystemState: systemState,
		RoutedAt: now.Format(time.RFC3339Nano), Router: w.Config.ConsumerName, SchemaVersion: "router.routed.v1",
	}
	if err := w.persistDecision(ctx, in, decision, now); err != nil {
		return err
	}
	if decision.Action == raise.ActionDefer {
		if err := w.persistOutbox(ctx, routed, decision, now); err != nil {
			return err
		}
	}
	if decision.Action == raise.ActionCloud {
		if err := w.publishCloudReq(ctx, routed); err != nil {
			return err
		}
	}
	if err := w.publishRouted(ctx, routed); err != nil {
		return err
	}
	w.State.Observe(in, decision.Action, now)
	_, _ = w.Runtime.SaveSystemSnapshot(ctx, w.Store, systemState, now)
	if err := w.Redis.XAck(ctx, w.Config.InferredStream, w.Config.ConsumerGroup, message.ID).Err(); err != nil {
		return fmt.Errorf("ack inferred message: %w", err)
	}
	return nil
}

func (w *Worker) persistDecision(ctx context.Context, in event.InferredEvent, decision raise.Decision, now time.Time) error {
	eventTS := parseTimePtr(in.EventTS)
	inferredAt := parseTimePtr(in.InferredAt)
	rawHash := ""
	if in.RawLog != "" {
		sum := sha256.Sum256([]byte(in.RawLog))
		rawHash = hex.EncodeToString(sum[:])
	}
	_, _, err := w.Store.CreateDecisionIfAbsent(ctx, store.RouterDecisionModel{
		EventID: in.EventID, TraceID: in.TraceID, SchemaVersion: in.SchemaVersion,
		Source: in.Source, Equipment: in.Equipment, EventTS: eventTS, InferredAt: inferredAt,
		TemplateID: in.TemplateID, RawLogHash: rawHash, EdgePred: in.EdgePred, Severity: in.Severity,
		Confidence: in.Confidence, EdgeUncertainty: in.Uncertainty, InferencePath: in.InferencePath,
		ModelVersion: in.ModelVersion, RuleVersion: in.RuleVersion, Action: decision.Action, Reason: decision.Reason,
		PolicyRef: decision.PolicyRef, Score: decision.Score, Threshold: decision.Threshold,
		CloudHealth: decision.CloudHealth, RaiseVersion: decision.RaiseVersion, LatencyMS: in.LatencyMS,
		RoutedAt: now, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return err
	}
	return nil
}

func (w *Worker) publishRouted(ctx context.Context, routed RoutedEvent) error {
	body, err := json.Marshal(routed)
	if err != nil {
		return err
	}
	return w.Redis.XAdd(ctx, &redis.XAddArgs{
		Stream: w.Config.RoutedStream, MaxLen: w.Config.StreamMaxLen, Approx: true,
		Values: map[string]any{"event_id": routed.Input.EventID, "action": routed.Decision.Action, "payload": string(body)},
	}).Err()
}

func (w *Worker) publishCloudReq(ctx context.Context, routed RoutedEvent) error {
	body, err := json.Marshal(routed)
	if err != nil {
		return err
	}
	return w.Redis.XAdd(ctx, &redis.XAddArgs{
		Stream: w.Config.CloudReqStream, MaxLen: w.Config.StreamMaxLen, Approx: true,
		Values: map[string]any{
			"event_id": routed.Input.EventID, "request_id": requestID(routed.Input.EventID),
			"idempotency_key": idempotencyKey(routed.Input.EventID), "payload_json": string(body), "source": "router_controller",
		},
	}).Err()
}

func (w *Worker) persistOutbox(ctx context.Context, routed RoutedEvent, decision raise.Decision, now time.Time) error {
	body, err := json.Marshal(routed)
	if err != nil {
		return err
	}
	decisionBody, err := json.Marshal(decision)
	if err != nil {
		return err
	}
	deadline := now.Add(10 * time.Minute)
	_, _, err = w.Store.CreateOutboxIfAbsent(ctx, store.CloudOutboxModel{
		EventID: routed.Input.EventID, RequestID: requestID(routed.Input.EventID), IdempotencyKey: idempotencyKey(routed.Input.EventID),
		PayloadJSON: string(body), RouteDecisionJSON: string(decisionBody), PriorityScore: decision.Score, DeferReason: decision.Reason,
		Status: store.OutboxStatusPending, MaxAttempts: w.Config.OutboxMaxAttempts, DeadlineAt: &deadline, CreatedAt: now, UpdatedAt: now,
	})
	return err
}

func (w *Worker) publishDLQAndAck(ctx context.Context, message redis.XMessage, payload string, cause error) error {
	_ = w.Redis.XAdd(ctx, &redis.XAddArgs{
		Stream: w.Config.DLQStream, MaxLen: w.Config.StreamMaxLen, Approx: true,
		Values: map[string]any{"source_stream": w.Config.InferredStream, "message_id": message.ID, "payload": payload, "error": cause.Error()},
	}).Err()
	return w.Redis.XAck(ctx, w.Config.InferredStream, w.Config.ConsumerGroup, message.ID).Err()
}

func (w *Worker) ensureConsumerGroup(ctx context.Context) error {
	if w.Redis == nil {
		return nil
	}
	err := w.Redis.XGroupCreateMkStream(ctx, w.Config.InferredStream, w.Config.ConsumerGroup, "0").Err()
	if err != nil && !containsBusyGroup(err) {
		return err
	}
	return nil
}

func decodeInferred(message redis.XMessage) (event.InferredEvent, string, error) {
	raw, _ := message.Values["payload"].(string)
	if raw == "" {
		body, _ := json.Marshal(message.Values)
		raw = string(body)
	}
	var in event.InferredEvent
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		return event.InferredEvent{}, raw, err
	}
	return in, raw, nil
}

func parseTimePtr(value string) *time.Time {
	if value == "" {
		return nil
	}
	if ts, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return &ts
	}
	return nil
}

func requestID(eventID string) string      { return "cloud-" + eventID }
func idempotencyKey(eventID string) string { return "router-cloud-" + eventID }

func containsBusyGroup(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToUpper(err.Error())
	return strings.Contains(msg, "BUSYGROUP") || strings.Contains(msg, "ALREADY EXISTS")
}

func (w *Worker) batch() int {
	if w.Config.WorkerBatch <= 0 {
		return 20
	}
	return w.Config.WorkerBatch
}

func (w *Worker) logf(format string, args ...any) {
	if w.Logger != nil {
		w.Logger.Printf(format, args...)
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
