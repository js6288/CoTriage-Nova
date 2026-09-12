package raise

import (
	"math"
	"strings"

	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/event"
)

type Engine struct {
	cfg Config
}

func New(cfg Config) *Engine {
	if cfg.PolicyRef == "" {
		def := DefaultConfig()
		cfg.PolicyRef = def.PolicyRef
	}
	if cfg.RaiseVersion == "" {
		def := DefaultConfig()
		cfg.RaiseVersion = def.RaiseVersion
	}
	if cfg.PathRisk == nil {
		cfg.PathRisk = DefaultConfig().PathRisk
	}
	if cfg.TotalLabels <= 0 {
		cfg.TotalLabels = DefaultConfig().TotalLabels
	}
	if cfg.TemplateTau <= 0 {
		cfg.TemplateTau = DefaultConfig().TemplateTau
	}
	if cfg.MaxFleetDevices <= 0 {
		cfg.MaxFleetDevices = DefaultConfig().MaxFleetDevices
	}
	if cfg.TargetLatencyMS <= 0 {
		cfg.TargetLatencyMS = DefaultConfig().TargetLatencyMS
	}
	if cfg.FailRateLimit <= 0 {
		cfg.FailRateLimit = DefaultConfig().FailRateLimit
	}
	if cfg.Temperature <= 0 {
		cfg.Temperature = DefaultConfig().Temperature
	}
	return &Engine{cfg: cfg}
}

func (engine *Engine) Decide(in event.InferredEvent, state SystemState, stats Stats, policy Policy) Decision {
	features := engine.features(in, stats)
	costs := engine.costs(state)
	pressures := engine.pressures(state)
	threshold := clamp01(engine.cfg.Threshold.Base +
		engine.cfg.Threshold.Queue*pressures.Queue +
		engine.cfg.Threshold.Budget*pressures.Budget +
		engine.cfg.Threshold.Network*pressures.Network -
		engine.cfg.Threshold.Audit*pressures.Audit)
	score := clamp01(engine.cfg.ScoreWeights.Value*features.CloudValue +
		engine.cfg.ScoreWeights.Gain*features.ExpectedGain -
		engine.cfg.ScoreWeights.Cost*costs.Total)
	sendable := engine.sendable(state)

	action, reason := engine.selectAction(in, policy, score, threshold, sendable)
	return Decision{
		EventID: in.EventID, Action: action, Reason: reason,
		PolicyRef: engine.cfg.PolicyRef, Score: score, Threshold: threshold,
		Features: features, Costs: costs, Pressures: pressures,
		CloudHealth: state.CloudHealth, RaiseVersion: engine.cfg.RaiseVersion,
		Sendable: sendable,
	}
}

func (engine *Engine) features(in event.InferredEvent, stats Stats) Features {
	risk := clamp01(float64(in.Severity) / 100)
	calibrated := engine.calibratedUncertainty(in, stats)
	novelty := math.Exp(-float64(maxInt(stats.TemplateFrequency, 0)) / engine.cfg.TemplateTau)
	disagreement := engine.disagreement(stats)
	fleetImpact := clamp01(float64(maxInt(stats.AffectedDevices, 0)) / float64(engine.cfg.MaxFleetDevices))
	pathRisk := engine.pathRisk(in.InferencePath)
	cloudValue := clamp01(
		engine.cfg.ValueWeights.Risk*risk +
			engine.cfg.ValueWeights.Uncertainty*calibrated +
			engine.cfg.ValueWeights.Novelty*novelty +
			engine.cfg.ValueWeights.Disagreement*disagreement +
			engine.cfg.ValueWeights.FleetImpact*fleetImpact +
			engine.cfg.ValueWeights.PathRisk*pathRisk)
	ucb := clamp01(engine.cfg.UCBAlpha * math.Sqrt(math.Max(stats.UCBUncertainty, 0)))
	expectedGain := clamp01(engine.cfg.ExpectedGainBias + 0.45*calibrated + 0.35*disagreement + 0.20*novelty + ucb)
	return Features{
		Risk: risk, CalibratedUncertainty: calibrated, ConformalSetSize: stats.ConformalSetSize,
		Novelty: novelty, HistoricalDisagreement: disagreement, FleetImpact: fleetImpact,
		PathRisk: pathRisk, ExpectedGain: expectedGain, UCBBonus: ucb, CloudValue: cloudValue,
	}
}

func (engine *Engine) calibratedUncertainty(in event.InferredEvent, stats Stats) float64 {
	conf := clampProbability(in.Confidence)
	logit := math.Log(conf / (1 - conf))
	pHat := 1 / (1 + math.Exp(-(logit / engine.cfg.Temperature)))
	modelUncertainty := 1 - pHat
	if in.Uncertainty > 0 {
		modelUncertainty = (modelUncertainty + clamp01(in.Uncertainty)) / 2
	}
	conformalSet := stats.ConformalSetSize
	if conformalSet <= 0 {
		conformalSet = 1
	}
	conformalUncertainty := clamp01(float64(conformalSet) / float64(engine.cfg.TotalLabels))
	eta := clamp01(engine.cfg.UncertaintyEta)
	return clamp01(eta*modelUncertainty + (1-eta)*conformalUncertainty)
}

func (engine *Engine) disagreement(stats Stats) float64 {
	alpha := engine.cfg.BetaPriorAlpha
	beta := engine.cfg.BetaPriorBeta
	if alpha <= 0 {
		alpha = 1
	}
	if beta <= 0 {
		beta = 3
	}
	corrections := float64(maxInt(stats.HistoricalCorrectionCount, 0))
	total := float64(maxInt(stats.HistoricalCloudCount, 0))
	return clamp01((corrections + alpha) / (total + alpha + beta))
}

func (engine *Engine) pathRisk(path string) float64 {
	if value, ok := engine.cfg.PathRisk[strings.ToUpper(strings.TrimSpace(path))]; ok {
		return clamp01(value)
	}
	return 0.50
}

func (engine *Engine) costs(state SystemState) Costs {
	remaining := math.Max(state.DayBudgetRemaining, 1)
	cloud := clamp01(engine.cfg.EstimatedTokens / remaining)
	latency := clamp01(math.Max(state.EstimatedLatencyMS-engine.cfg.TargetLatencyMS, 0) / engine.cfg.TargetLatencyMS)
	network := engine.networkPressure(state)
	privacy := clamp01(state.PrivacyCost)
	total := clamp01(engine.cfg.CostWeights.Cloud*cloud +
		engine.cfg.CostWeights.Latency*latency +
		engine.cfg.CostWeights.Network*network +
		engine.cfg.CostWeights.Privacy*privacy)
	return Costs{Cloud: cloud, Latency: latency, Network: network, Privacy: privacy, Total: total}
}

func (engine *Engine) pressures(state SystemState) Pressures {
	queueMax := state.QueueMax
	if queueMax <= 0 {
		queueMax = 1000
	}
	queue := clamp01(float64(maxInt(state.CloudQueueLag, 0)+maxInt(state.OutboxLag, 0)) / float64(queueMax))
	budget := 0.0
	if state.DayBudgetRemaining <= 0 {
		budget = 1
	} else {
		budget = clamp01(1 / math.Sqrt(state.DayBudgetRemaining))
	}
	network := engine.networkPressure(state)
	audit := clamp01(math.Max(state.MinCloudRatioForAudit-state.RecentCloudRatio, 0))
	return Pressures{Queue: queue, Budget: budget, Network: network, Audit: audit}
}

func (engine *Engine) networkPressure(state SystemState) float64 {
	rtt := clamp01(state.RTTMS / 1000)
	fail := clamp01(state.FailRate)
	health := 0.0
	if strings.ToUpper(strings.TrimSpace(state.CloudHealth)) != "UP" {
		health = 1
	}
	return clamp01(engine.cfg.NetworkWeights.RTT*rtt + engine.cfg.NetworkWeights.Fail*fail + engine.cfg.NetworkWeights.Health*health)
}

func (engine *Engine) sendable(state SystemState) bool {
	if strings.ToUpper(strings.TrimSpace(state.CloudHealth)) != "UP" {
		return false
	}
	if state.RateLimitRemaining <= 0 {
		return false
	}
	if state.MinuteBudgetRemaining <= 0 || state.DayBudgetRemaining <= 0 {
		return false
	}
	return state.FailRate <= engine.cfg.FailRateLimit
}

func (engine *Engine) selectAction(in event.InferredEvent, policy Policy, score, threshold float64, sendable bool) (string, string) {
	if matches(policy.DenyCloudEdgePreds, in.EdgePred) || matches(policy.DenyCloudSources, in.Source) {
		return ActionLocal, ReasonDenyCloudPolicy
	}
	if matches(policy.ForceLocalEdgePreds, in.EdgePred) {
		return ActionLocal, ReasonForceLocalPolicy
	}
	if matches(policy.ForceCloudEdgePreds, in.EdgePred) || matches(policy.ForceCloudTemplates, in.TemplateID) {
		if sendable {
			return ActionCloud, ReasonForceCloudSendable
		}
		return ActionDefer, ReasonForceCloudDefer
	}
	if score < threshold {
		return ActionLocal, ReasonLowScore
	}
	if sendable {
		return ActionCloud, ReasonHighScoreSendable
	}
	return ActionDefer, ReasonHighScoreDefer
}

func matches(values map[string]bool, value string) bool {
	if len(values) == 0 {
		return false
	}
	return values[strings.ToUpper(strings.TrimSpace(value))]
}
