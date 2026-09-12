package runtime

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/config"
)

func TestTokenBucketsAllowAndRefill(t *testing.T) {
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	buckets := NewTokenBuckets(2, 10, 100, 2, now)
	if !buckets.Allow(1, now) || !buckets.Allow(1, now) {
		t.Fatalf("expected first two requests to be allowed")
	}
	if buckets.Allow(1, now) {
		t.Fatalf("expected third request to be rejected")
	}
	later := now.Add(30 * time.Second)
	buckets.RefillTo(later)
	if buckets.Minute.Tokens <= 0 {
		t.Fatalf("expected minute bucket to refill")
	}
}

func TestHealthProbeSmoothsFailures(t *testing.T) {
	okServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer okServer.Close()
	probe := NewHealthProbe(okServer.URL, time.Second, 4)
	probe.Probe(nil)
	probe.Probe(nil)
	health, _, failRate := probe.Snapshot()
	if health != CloudHealthUp || failRate != 0 {
		t.Fatalf("health/failRate = %s/%f, want UP/0", health, failRate)
	}
	probe.Add(HealthSample{OK: false})
	probe.Add(HealthSample{OK: false})
	health, _, failRate = probe.Snapshot()
	if health != CloudHealthDown || failRate < 0.5 {
		t.Fatalf("health/failRate = %s/%f, want DOWN/>=0.5", health, failRate)
	}
}

func TestBuilderProducesSystemState(t *testing.T) {
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	cfg := config.Config{
		MinuteBudget: 10, HourBudget: 100, DayBudget: 1000, RateLimit: 10,
		QueuePressureMax: 100, MinCloudRatioForAudit: 0.1, EstimatedCloudLatency: time.Second,
	}
	buckets := NewTokenBuckets(cfg.MinuteBudget, cfg.HourBudget, cfg.DayBudget, cfg.RateLimit, now)
	probe := NewHealthProbe("", time.Second, 4)
	builder := NewBuilder(cfg, nil, probe, &buckets)
	state, err := builder.Build(nil, 0.05, 0.2, now)
	if err != nil {
		t.Fatalf("build state: %v", err)
	}
	if state.CloudHealth != CloudHealthUp {
		t.Fatalf("cloud health = %s, want UP", state.CloudHealth)
	}
	if state.DayBudgetRemaining != 1000 {
		t.Fatalf("day budget = %f, want 1000", state.DayBudgetRemaining)
	}
	if state.MinCloudRatioForAudit != 0.1 || state.PrivacyCost != 0.2 {
		t.Fatalf("unexpected audit/privacy state: %+v", state)
	}
}
