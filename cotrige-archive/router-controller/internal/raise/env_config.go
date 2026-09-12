package raise

import cfg "github.com/pangjian-pj/cotriage-dev/router-controller/internal/config"

func LoadConfigFromEnv() Config {
	conf := DefaultConfig()
	conf.PolicyRef = cfg.Getenv("RAISE_POLICY_REF", conf.PolicyRef)
	conf.RaiseVersion = cfg.Getenv("RAISE_VERSION", conf.RaiseVersion)

	conf.ValueWeights.Risk = cfg.GetenvFloat("RAISE_WEIGHT_RISK", conf.ValueWeights.Risk)
	conf.ValueWeights.Uncertainty = cfg.GetenvFloat("RAISE_WEIGHT_UNCERTAINTY", conf.ValueWeights.Uncertainty)
	conf.ValueWeights.Novelty = cfg.GetenvFloat("RAISE_WEIGHT_NOVELTY", conf.ValueWeights.Novelty)
	conf.ValueWeights.Disagreement = cfg.GetenvFloat("RAISE_WEIGHT_DISAGREEMENT", conf.ValueWeights.Disagreement)
	conf.ValueWeights.FleetImpact = cfg.GetenvFloat("RAISE_WEIGHT_FLEET_IMPACT", conf.ValueWeights.FleetImpact)
	conf.ValueWeights.PathRisk = cfg.GetenvFloat("RAISE_WEIGHT_PATH_RISK", conf.ValueWeights.PathRisk)

	conf.CostWeights.Cloud = cfg.GetenvFloat("RAISE_COST_CLOUD", conf.CostWeights.Cloud)
	conf.CostWeights.Latency = cfg.GetenvFloat("RAISE_COST_LATENCY", conf.CostWeights.Latency)
	conf.CostWeights.Network = cfg.GetenvFloat("RAISE_COST_NETWORK", conf.CostWeights.Network)
	conf.CostWeights.Privacy = cfg.GetenvFloat("RAISE_COST_PRIVACY", conf.CostWeights.Privacy)

	conf.ScoreWeights.Value = cfg.GetenvFloat("RAISE_SCORE_VALUE", conf.ScoreWeights.Value)
	conf.ScoreWeights.Gain = cfg.GetenvFloat("RAISE_SCORE_GAIN", conf.ScoreWeights.Gain)
	conf.ScoreWeights.Cost = cfg.GetenvFloat("RAISE_SCORE_COST", conf.ScoreWeights.Cost)

	conf.Threshold.Base = cfg.GetenvFloat("RAISE_THRESHOLD_BASE", conf.Threshold.Base)
	conf.Threshold.Queue = cfg.GetenvFloat("RAISE_THRESHOLD_QUEUE", conf.Threshold.Queue)
	conf.Threshold.Budget = cfg.GetenvFloat("RAISE_THRESHOLD_BUDGET", conf.Threshold.Budget)
	conf.Threshold.Network = cfg.GetenvFloat("RAISE_THRESHOLD_NETWORK", conf.Threshold.Network)
	conf.Threshold.Audit = cfg.GetenvFloat("RAISE_THRESHOLD_AUDIT", conf.Threshold.Audit)

	conf.Temperature = cfg.GetenvFloat("RAISE_TEMPERATURE", conf.Temperature)
	conf.UncertaintyEta = cfg.GetenvFloat("RAISE_UNCERTAINTY_ETA", conf.UncertaintyEta)
	conf.TemplateTau = cfg.GetenvFloat("RAISE_TEMPLATE_TAU", conf.TemplateTau)
	conf.UCBAlpha = cfg.GetenvFloat("RAISE_UCB_ALPHA", conf.UCBAlpha)
	conf.BetaPriorAlpha = cfg.GetenvFloat("RAISE_BETA_PRIOR_ALPHA", conf.BetaPriorAlpha)
	conf.BetaPriorBeta = cfg.GetenvFloat("RAISE_BETA_PRIOR_BETA", conf.BetaPriorBeta)
	conf.EstimatedTokens = cfg.GetenvFloat("RAISE_ESTIMATED_TOKENS", conf.EstimatedTokens)
	conf.TargetLatencyMS = cfg.GetenvFloat("RAISE_TARGET_LATENCY_MS", conf.TargetLatencyMS)
	conf.FailRateLimit = cfg.GetenvFloat("RAISE_FAIL_RATE_LIMIT", conf.FailRateLimit)
	conf.ExpectedGainBias = cfg.GetenvFloat("RAISE_EXPECTED_GAIN_BIAS", conf.ExpectedGainBias)
	conf.MaxFleetDevices = cfg.GetenvInt("RAISE_MAX_FLEET_DEVICES", conf.MaxFleetDevices)
	conf.TotalLabels = cfg.GetenvInt("RAISE_TOTAL_LABELS", conf.TotalLabels)

	conf.PathRisk["RULE"] = cfg.GetenvFloat("RAISE_PATH_RISK_RULE", conf.PathRisk["RULE"])
	conf.PathRisk["CACHE"] = cfg.GetenvFloat("RAISE_PATH_RISK_CACHE", conf.PathRisk["CACHE"])
	conf.PathRisk["MODEL"] = cfg.GetenvFloat("RAISE_PATH_RISK_MODEL", conf.PathRisk["MODEL"])
	conf.PathRisk["EDGE_LLM"] = cfg.GetenvFloat("RAISE_PATH_RISK_EDGE_LLM", conf.PathRisk["EDGE_LLM"])
	conf.PathRisk["FALLBACK"] = cfg.GetenvFloat("RAISE_PATH_RISK_FALLBACK", conf.PathRisk["FALLBACK"])

	return conf
}
