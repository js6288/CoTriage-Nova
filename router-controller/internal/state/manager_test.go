package state

import (
	"testing"
	"time"

	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/event"
	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/raise"
)

func TestWindowPrunesExpiredObservations(t *testing.T) {
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	window := NewWindow(time.Minute)
	window.Add(Observation{TemplateID: "tpl", Source: "a", SeenAt: now.Add(-2 * time.Minute)}, now.Add(-2*time.Minute))
	window.Add(Observation{TemplateID: "tpl", Source: "b", SeenAt: now}, now)
	if got := window.TemplateFrequency("tpl", now); got != 1 {
		t.Fatalf("frequency = %d, want 1", got)
	}
	if got := window.AffectedDevices("tpl", now); got != 1 {
		t.Fatalf("affected devices = %d, want 1", got)
	}
}

func TestFeedbackUpdatesTemplateStatsAndGain(t *testing.T) {
	manager := NewManager(10*time.Minute, 7, 0.1)
	in := event.InferredEvent{TemplateID: "tpl", Severity: 80}
	features := raise.Features{
		CalibratedUncertainty: 0.7, Novelty: 0.8, HistoricalDisagreement: 0.2,
		FleetImpact: 0.3, PathRisk: 0.4,
	}
	before, _ := manager.GainModel.Predict(GainFeatures(in, features))
	manager.ApplyFeedback(in, true, 1.0, features)
	after, _ := manager.GainModel.Predict(GainFeatures(in, features))
	if after <= before {
		t.Fatalf("expected gain to increase, before=%f after=%f", before, after)
	}
	stat := manager.TemplateStats.Get("tpl")
	if stat.CloudCount != 1 || stat.CorrectionCount != 1 {
		t.Fatalf("unexpected template stats: %+v", stat)
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	manager := NewManager(10*time.Minute, 7, 0.1)
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	in := event.InferredEvent{TemplateID: "tpl", Source: "node-1", EdgePred: "NET_ERROR"}
	manager.Observe(in, raise.ActionCloud, now)
	manager.TemplateStats.RecordFeedback("tpl", true)
	body, err := manager.SnapshotJSON()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	restored, err := RestoreManager(body)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if got := restored.Window.TemplateFrequency("tpl", now); got != 1 {
		t.Fatalf("restored frequency = %d, want 1", got)
	}
	if got := restored.TemplateStats.Get("tpl").CorrectionCount; got != 1 {
		t.Fatalf("restored correction count = %d, want 1", got)
	}
	if got := restored.RecentCloudRatio(); got != 1 {
		t.Fatalf("restored cloud ratio = %f, want 1", got)
	}
}
