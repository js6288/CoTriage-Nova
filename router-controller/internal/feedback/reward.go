package feedback

import (
	"strings"

	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/store"
)

const (
	RewardCorrectedEdge      = "corrected_edge"
	RewardActionableInfo     = "actionable_info"
	RewardConfirmedHighRisk  = "confirmed_high_risk"
	RewardNoAdditionalSignal = "no_additional_signal"
)

type RewardResult struct {
	Reward    float64
	Reason    string
	Corrected bool
}

func ClassifyReward(item store.CloudFeedbackModel) RewardResult {
	if item.CorrectedEdge || differs(item.EdgePred, item.CloudPred) {
		return RewardResult{Reward: 1.0, Reason: RewardCorrectedEdge, Corrected: true}
	}
	if item.AddedActionableInfo || hasText(item.RootCause) || hasText(item.Recommendation) {
		return RewardResult{Reward: 0.7, Reason: RewardActionableInfo}
	}
	if item.ConfirmedHighRisk {
		return RewardResult{Reward: 0.3, Reason: RewardConfirmedHighRisk}
	}
	return RewardResult{Reward: 0, Reason: RewardNoAdditionalSignal}
}

func differs(left, right string) bool {
	left = strings.TrimSpace(strings.ToLower(left))
	right = strings.TrimSpace(strings.ToLower(right))
	return left != "" && right != "" && left != right
}

func hasText(value string) bool {
	return strings.TrimSpace(value) != ""
}
