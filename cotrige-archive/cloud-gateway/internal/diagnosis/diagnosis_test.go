package diagnosis

import (
	"strings"
	"testing"

	"github.com/pangjian-pj/cotriage-dev/cloud-gateway/internal/event"
)

func TestRedact(t *testing.T) {
	got := Redact("connect 10.0.0.2 user=a@example.com token=abcdef")
	for _, sensitive := range []string{"10.0.0.2", "a@example.com", "abcdef"} {
		if strings.Contains(got, sensitive) {
			t.Fatalf("sensitive value remained: %s", got)
		}
	}
}

func TestFeedback(t *testing.T) {
	f := Feedback(event.InferredEvent{EdgePred: "SERVICE_ERROR", Severity: 90, Confidence: .4}, event.Diagnosis{Pred: "NET_ERROR", RootCause: "closed", Recommendation: "restart", Confidence: .9}, event.Usage{LatencyMS: 1000, TotalTokens: 500}, 5000, 2000)
	if !f.CorrectedEdge || !f.AddedActionableInfo || !f.ConfirmedHighRisk || f.RewardReason != "CORRECTED_EDGE" || f.Reward <= 0 {
		t.Fatalf("unexpected feedback: %+v", f)
	}
}
