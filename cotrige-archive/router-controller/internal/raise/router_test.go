package raise

import (
	"testing"

	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/event"
)

func baseEvent() event.InferredEvent {
	return event.InferredEvent{
		EventID: "evt-1", Source: "host-1", TemplateID: "tpl-1",
		EdgePred: "NET_ERROR", Severity: 80, Confidence: 0.60,
		Uncertainty: 0.40, InferencePath: "MODEL",
	}
}

func healthyState() SystemState {
	return SystemState{
		CloudHealth: "UP", RTTMS: 80, FailRate: 0.01,
		RateLimitRemaining: 10, MinuteBudgetRemaining: 10, DayBudgetRemaining: 10000,
		QueueMax: 1000, MinCloudRatioForAudit: 0.10, RecentCloudRatio: 0.10,
		EstimatedLatencyMS: 1000,
	}
}

func highValueStats() Stats {
	return Stats{
		TemplateFrequency: 1, HistoricalCloudCount: 10, HistoricalCorrectionCount: 6,
		AffectedDevices: 8, ConformalSetSize: 4, UCBUncertainty: 0.25,
	}
}

func TestHighRiskEventRoutesToCloudWhenSendable(t *testing.T) {
	engine := New(DefaultConfig())
	decision := engine.Decide(baseEvent(), healthyState(), highValueStats(), Policy{})
	if decision.Action != ActionCloud {
		t.Fatalf("action = %s, want %s; score=%f threshold=%f", decision.Action, ActionCloud, decision.Score, decision.Threshold)
	}
}

func TestLowRiskEventRoutesLocal(t *testing.T) {
	engine := New(DefaultConfig())
	evt := baseEvent()
	evt.EdgePred = "NORMAL"
	evt.Severity = 5
	evt.Confidence = 0.99
	evt.Uncertainty = 0.01
	evt.InferencePath = "RULE"
	decision := engine.Decide(evt, healthyState(), Stats{TemplateFrequency: 100, ConformalSetSize: 1}, Policy{})
	if decision.Action != ActionLocal {
		t.Fatalf("action = %s, want %s; score=%f threshold=%f", decision.Action, ActionLocal, decision.Score, decision.Threshold)
	}
}

func TestHighValueEventDefersWhenCloudDown(t *testing.T) {
	engine := New(DefaultConfig())
	evt := baseEvent()
	evt.Severity = 99
	evt.Confidence = 0.20
	evt.Uncertainty = 0.80
	evt.InferencePath = "FALLBACK"
	state := healthyState()
	state.CloudHealth = "DOWN"
	stats := highValueStats()
	stats.TemplateFrequency = 0
	stats.HistoricalCloudCount = 10
	stats.HistoricalCorrectionCount = 10
	stats.AffectedDevices = 20
	stats.ConformalSetSize = 8
	stats.UCBUncertainty = 1
	decision := engine.Decide(evt, state, stats, Policy{})
	if decision.Action != ActionDefer {
		t.Fatalf("action = %s, want %s", decision.Action, ActionDefer)
	}
}

func TestDenyCloudPolicyForcesLocal(t *testing.T) {
	engine := New(DefaultConfig())
	policy := Policy{DenyCloudEdgePreds: map[string]bool{"NET_ERROR": true}}
	decision := engine.Decide(baseEvent(), healthyState(), highValueStats(), policy)
	if decision.Action != ActionLocal || decision.Reason != ReasonDenyCloudPolicy {
		t.Fatalf("action/reason = %s/%s, want %s/%s", decision.Action, decision.Reason, ActionLocal, ReasonDenyCloudPolicy)
	}
}

func TestForceCloudDefersWhenNotSendable(t *testing.T) {
	engine := New(DefaultConfig())
	state := healthyState()
	state.RateLimitRemaining = 0
	policy := Policy{ForceCloudTemplates: map[string]bool{"TPL-1": true}}
	decision := engine.Decide(baseEvent(), state, Stats{}, policy)
	if decision.Action != ActionDefer || decision.Reason != ReasonForceCloudDefer {
		t.Fatalf("action/reason = %s/%s, want %s/%s", decision.Action, decision.Reason, ActionDefer, ReasonForceCloudDefer)
	}
}
