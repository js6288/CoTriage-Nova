package feedback

import (
	"testing"

	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/store"
)

func TestClassifyReward(t *testing.T) {
	cases := []struct {
		name   string
		item   store.CloudFeedbackModel
		reward float64
		reason string
	}{
		{
			name:   "corrected edge",
			item:   store.CloudFeedbackModel{EdgePred: "A", CloudPred: "B"},
			reward: 1.0,
			reason: RewardCorrectedEdge,
		},
		{
			name:   "actionable info",
			item:   store.CloudFeedbackModel{EdgePred: "A", CloudPred: "A", Recommendation: "restart service"},
			reward: 0.7,
			reason: RewardActionableInfo,
		},
		{
			name:   "confirmed high risk",
			item:   store.CloudFeedbackModel{ConfirmedHighRisk: true},
			reward: 0.3,
			reason: RewardConfirmedHighRisk,
		},
		{
			name:   "no signal",
			item:   store.CloudFeedbackModel{},
			reward: 0,
			reason: RewardNoAdditionalSignal,
		},
	}
	for _, tc := range cases {
		got := ClassifyReward(tc.item)
		if got.Reward != tc.reward || got.Reason != tc.reason {
			t.Fatalf("%s reward = %+v, want %f/%s", tc.name, got, tc.reward, tc.reason)
		}
	}
}
