package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/pangjian-pj/cotriage-dev/cloud-gateway/internal/config"
	"github.com/pangjian-pj/cotriage-dev/cloud-gateway/internal/contextlog"
	"github.com/pangjian-pj/cotriage-dev/cloud-gateway/internal/diagnosis"
	"github.com/pangjian-pj/cotriage-dev/cloud-gateway/internal/event"
	"github.com/pangjian-pj/cotriage-dev/cloud-gateway/internal/provider"
	"github.com/pangjian-pj/cotriage-dev/cloud-gateway/internal/store"
	"github.com/redis/go-redis/v9"
)

type Worker struct {
	Cfg        config.Config
	Redis      *redis.Client
	Store      *store.Store
	Providers  *provider.Manager
	Client     provider.Client
	Aggregator contextlog.Aggregator
	Logger     *log.Logger
}

func (w *Worker) Run(ctx context.Context) error {
	sem := make(chan struct{}, max(w.Cfg.Concurrency, 1))
	groupReady := false
	for {
		if !groupReady {
			if e := w.ensureGroup(ctx); e != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				w.logf("ensure consumer group failed: %v; retrying in %s", e, w.retryDelay())
				if e := waitForRetry(ctx, w.retryDelay()); e != nil {
					return e
				}
				continue
			}
			groupReady = true
		}

		xs, e := w.Redis.XReadGroup(ctx, &redis.XReadGroupArgs{Group: w.Cfg.Group, Consumer: w.Cfg.Consumer, Streams: []string{w.Cfg.RequestStream, ">"}, Count: int64(w.Cfg.Batch), Block: w.Cfg.BlockTimeout}).Result()
		if e == redis.Nil {
			continue
		}
		if e != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			w.logf("read request stream failed: %v; retrying in %s", e, w.retryDelay())
			groupReady = false
			if e := waitForRetry(ctx, w.retryDelay()); e != nil {
				return e
			}
			continue
		}
		for _, stream := range xs {
			for _, msg := range stream.Messages {
				sem <- struct{}{}
				go func(m redis.XMessage) {
					defer func() { <-sem }()
					if e := w.Handle(ctx, m); e != nil {
						w.logf("message=%s failed: %v", m.ID, e)
					}
				}(msg)
			}
		}
	}
}

func (w *Worker) retryDelay() time.Duration {
	if w.Cfg.RedisRetryDelay > 0 {
		return w.Cfg.RedisRetryDelay
	}
	return 3 * time.Second
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func (w *Worker) Handle(ctx context.Context, m redis.XMessage) error {
	req, e := decode(m)
	if e != nil {
		return w.dlq(ctx, m, "BAD_PAYLOAD", e)
	}
	exists, e := w.Store.Exists(ctx, req.EventID)
	if e != nil {
		return e
	}
	if exists {
		saved, getErr := w.Store.Get(ctx, req.EventID)
		if getErr != nil {
			return getErr
		}
		if e = w.publishResponse(ctx, req, saved.FeedbackJSON); e != nil {
			return e
		}
		return w.Redis.XAck(ctx, w.Cfg.RequestStream, w.Cfg.Group, m.ID).Err()
	}
	cfg, e := w.Providers.Get()
	if e != nil {
		return w.dlq(ctx, m, "PROVIDER_CONFIG_MISSING", e)
	}
	cx := w.Aggregator.Build(ctx, req.Routed.Input)
	system, user := diagnosis.Prompt(req.Routed, cx.RelatedEvents, w.Cfg.RawLogMaxChars)
	d, u, e := w.Client.Diagnose(ctx, *cfg, system, user)
	if e != nil {
		if retryable(e) {
			return e
		}
		return w.dlq(ctx, m, errorCode(e), e)
	}
	fb := diagnosis.Feedback(req.Routed.Input, d, u, w.Cfg.LatencySLAMS, w.Cfg.TokenBudget)
	resp := event.Response{SchemaVersion: "cloud.response.v1", EventID: req.EventID, RequestID: req.RequestID, IdempotencyKey: req.IdempotencyKey, Status: "OK", Edge: &event.EdgeSummary{Pred: req.Routed.Input.EdgePred, Confidence: req.Routed.Input.Confidence, Severity: req.Routed.Input.Severity, AdviceCode: req.Routed.Input.AdviceCode}, Cloud: &d, Feedback: &fb, Usage: &u, Context: cx, ProcessedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	body, _ := json.Marshal(resp)
	f := store.Feedback{EventID: req.EventID, RequestID: req.RequestID, IdempotencyKey: req.IdempotencyKey, Source: req.Routed.Input.Source, Equipment: req.Routed.Input.Equipment, TemplateID: req.Routed.Input.TemplateID, EdgePred: req.Routed.Input.EdgePred, EdgeConfidence: req.Routed.Input.Confidence, RouteAction: req.Routed.Decision.Action, RouteScore: req.Routed.Decision.Score, RouteThreshold: req.Routed.Decision.Threshold, CloudModel: cfg.Model, CloudPred: d.Pred, RootCause: d.RootCause, Recommendation: d.Recommendation, Explanation: d.Explanation, CloudConfidence: d.Confidence, CloudLatencyMS: u.LatencyMS, TokenUsage: u.TotalTokens, Cost: u.Cost, Reward: fb.Reward, RewardReason: fb.RewardReason, CorrectedEdge: fb.CorrectedEdge, AddedActionableInfo: fb.AddedActionableInfo, ConfirmedHighRisk: fb.ConfirmedHighRisk, FeedbackJSON: string(body), ProcessedByRouter: false, CreatedAt: time.Now().UTC()}
	if e = w.Store.Insert(ctx, f); e != nil {
		return e
	}
	if e = w.publishResponse(ctx, req, string(body)); e != nil {
		return e
	}
	return w.Redis.XAck(ctx, w.Cfg.RequestStream, w.Cfg.Group, m.ID).Err()
}
func (w *Worker) publishResponse(ctx context.Context, req event.Request, body string) error {
	return w.Redis.XAdd(ctx, &redis.XAddArgs{Stream: w.Cfg.ResponseStream, MaxLen: w.Cfg.StreamMaxLen, Approx: true, Values: map[string]any{"event_id": req.EventID, "request_id": req.RequestID, "status": "OK", "payload_json": body}}).Err()
}
func decode(m redis.XMessage) (event.Request, error) {
	get := func(k string) string {
		switch v := m.Values[k].(type) {
		case string:
			return v
		case []byte:
			return string(v)
		default:
			return fmt.Sprint(v)
		}
	}
	r := event.Request{EventID: get("event_id"), RequestID: get("request_id"), IdempotencyKey: get("idempotency_key")}
	if r.EventID == "" || r.RequestID == "" || r.IdempotencyKey == "" {
		return r, errors.New("required message field missing")
	}
	if e := json.Unmarshal([]byte(get("payload_json")), &r.Routed); e != nil {
		return r, e
	}
	if r.Routed.Input.EventID != r.EventID {
		return r, errors.New("event_id mismatch")
	}
	return r, nil
}
func (w *Worker) dlq(ctx context.Context, m redis.XMessage, code string, cause error) error {
	failed := event.Response{
		SchemaVersion: "cloud.response.v1",
		EventID:       fmt.Sprint(m.Values["event_id"]), RequestID: fmt.Sprint(m.Values["request_id"]),
		IdempotencyKey: fmt.Sprint(m.Values["idempotency_key"]), Status: "FAILED",
		ErrorCode: code, ErrorMessage: cause.Error(), Retryable: false,
		ProcessedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	if body, err := json.Marshal(failed); err == nil {
		if err = w.Redis.XAdd(ctx, &redis.XAddArgs{Stream: w.Cfg.ResponseStream, MaxLen: w.Cfg.StreamMaxLen, Approx: true, Values: map[string]any{"event_id": failed.EventID, "request_id": failed.RequestID, "status": "FAILED", "payload_json": string(body)}}).Err(); err != nil {
			return err
		}
	}
	e := w.Redis.XAdd(ctx, &redis.XAddArgs{Stream: w.Cfg.DLQStream, MaxLen: w.Cfg.StreamMaxLen, Approx: true, Values: map[string]any{"source_stream": w.Cfg.RequestStream, "message_id": m.ID, "event_id": fmt.Sprint(m.Values["event_id"]), "request_id": fmt.Sprint(m.Values["request_id"]), "error_code": code, "error": cause.Error()}}).Err()
	if e != nil {
		return e
	}
	return w.Redis.XAck(ctx, w.Cfg.RequestStream, w.Cfg.Group, m.ID).Err()
}
func (w *Worker) ensureGroup(ctx context.Context) error {
	e := w.Redis.XGroupCreateMkStream(ctx, w.Cfg.RequestStream, w.Cfg.Group, "0").Err()
	if e != nil && !strings.Contains(strings.ToUpper(e.Error()), "BUSYGROUP") {
		return e
	}
	return nil
}
func retryable(e error) bool {
	s := e.Error()
	return strings.Contains(s, "LLM_TIMEOUT") || strings.Contains(s, "RATE_LIMIT") || strings.Contains(s, "PROVIDER_5XX")
}
func errorCode(e error) string {
	if i := strings.Index(e.Error(), ":"); i > 0 {
		return e.Error()[:i]
	}
	return e.Error()
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func (w *Worker) logf(f string, a ...any) {
	if w.Logger != nil {
		w.Logger.Printf(f, a...)
	} else {
		log.Printf(f, a...)
	}
}
