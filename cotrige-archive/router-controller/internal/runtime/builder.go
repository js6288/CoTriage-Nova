package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/config"
	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/raise"
	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/store"
)

const (
	SystemSnapshotType = "system_state"
	SystemSnapshotKey  = "default"
	SystemVersion      = "raise-runtime-v1"
)

var ErrStoreUnavailable = errors.New("runtime snapshot store unavailable")

type Builder struct {
	QueueWatcher *QueueWatcher
	HealthProbe  *HealthProbe
	Buckets      *TokenBuckets
	Config       config.Config
}

type SystemSnapshot struct {
	State     raise.SystemState `json:"state"`
	Buckets   TokenBuckets      `json:"buckets"`
	CreatedAt time.Time         `json:"created_at"`
}

func NewBuilder(cfg config.Config, queue *QueueWatcher, probe *HealthProbe, buckets *TokenBuckets) *Builder {
	if buckets == nil {
		now := time.Now().UTC()
		value := NewTokenBuckets(cfg.MinuteBudget, cfg.HourBudget, cfg.DayBudget, cfg.RateLimit, now)
		buckets = &value
	}
	if probe == nil {
		probe = NewHealthProbe(cfg.CloudHealthURL, cfg.CloudHealthTimeout, cfg.CloudHealthWindow)
	}
	return &Builder{Config: cfg, QueueWatcher: queue, HealthProbe: probe, Buckets: buckets}
}

func (b *Builder) Build(ctx context.Context, recentCloudRatio float64, privacyCost float64, now time.Time) (raise.SystemState, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	b.Buckets.RefillTo(now)
	healthSample := b.HealthProbe.Probe(ctx)
	_ = healthSample
	health, rttMS, failRate := b.HealthProbe.Snapshot()
	queueSnap := QueueSnapshot{}
	var err error
	if b.QueueWatcher != nil {
		queueSnap, err = b.QueueWatcher.Snapshot(ctx)
	}
	state := raise.SystemState{
		CloudHealth:           health,
		RTTMS:                 rttMS,
		FailRate:              failRate,
		RateLimitRemaining:    int(b.Buckets.Rate.Tokens),
		MinuteBudgetRemaining: b.Buckets.Minute.Tokens,
		HourBudgetRemaining:   b.Buckets.Hour.Tokens,
		DayBudgetRemaining:    b.Buckets.Day.Tokens,
		CloudQueueLag:         int(queueSnap.CloudReqLength + queueSnap.InferredPending),
		OutboxLag:             int(queueSnap.OutboxPending),
		QueueMax:              b.Config.QueuePressureMax,
		RecentCloudRatio:      recentCloudRatio,
		MinCloudRatioForAudit: b.Config.MinCloudRatioForAudit,
		EstimatedLatencyMS:    float64(b.Config.EstimatedCloudLatency.Milliseconds()),
		PrivacyCost:           privacyCost,
	}
	return state, err
}

func (b *Builder) ConsumeCloudBudget(cost float64, now time.Time) bool {
	return b.Buckets.Allow(cost, now)
}

func (b *Builder) SaveSystemSnapshot(ctx context.Context, st *store.Store, state raise.SystemState, now time.Time) (store.RouterStateSnapshotModel, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if st == nil && b.QueueWatcher != nil {
		st = b.QueueWatcher.Store
	}
	if st == nil {
		return store.RouterStateSnapshotModel{}, ErrStoreUnavailable
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	payload, err := json.Marshal(SystemSnapshot{
		State:     state,
		Buckets:   *b.Buckets,
		CreatedAt: now,
	})
	if err != nil {
		return store.RouterStateSnapshotModel{}, err
	}
	return st.SaveStateSnapshot(ctx, store.RouterStateSnapshotModel{
		SnapshotType: SystemSnapshotType,
		SnapshotKey:  SystemSnapshotKey,
		PayloadJSON:  string(payload),
		Version:      SystemVersion,
		IsActive:     true,
		CreatedAt:    now,
	})
}
