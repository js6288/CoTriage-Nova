package infer

import "testing"

func TestRuleEngineUsesHighestPriorityMatch(t *testing.T) {
	engine, err := NewRuleEngine("test-v1", []Rule{
		{ID: "low", Pattern: "failed", EdgePred: PredAppError, Severity: 50, Confidence: 0.8, Priority: 1, Enabled: true},
		{ID: "high", Pattern: "authentication failed", EdgePred: PredSecurity, Severity: 80, Confidence: 0.95, Priority: 10, Enabled: true},
	})
	if err != nil {
		t.Fatalf("NewRuleEngine() error = %v", err)
	}

	result, ok := engine.Match("authentication failed for user admin")
	if !ok {
		t.Fatal("Match() did not find a rule")
	}
	if result.EdgePred != PredSecurity {
		t.Fatalf("Match().EdgePred = %q, want %q", result.EdgePred, PredSecurity)
	}
}

func TestRuleEngineRejectsInvalidRule(t *testing.T) {
	_, err := NewRuleEngine("test-v1", []Rule{
		{ID: "bad", Pattern: "(", EdgePred: PredUnknown, Severity: 50, Confidence: 0.5, Enabled: true},
	})
	if err == nil {
		t.Fatal("NewRuleEngine() error = nil, want invalid regexp error")
	}
}

func TestDefaultDiskRule(t *testing.T) {
	engine, err := LoadRules("")
	if err != nil {
		t.Fatalf("LoadRules() error = %v", err)
	}
	result, ok := engine.Match("write failed: no space left on device")
	if !ok || result.EdgePred != PredDiskError || result.Severity < 90 {
		t.Fatalf("Match() = %+v, %v; want high-severity disk error", result, ok)
	}
}

