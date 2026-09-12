package raise

func DefaultConfig() Config {
	return Config{
		PolicyRef:    "raise-v1",
		RaiseVersion: "raise-config-v1",
		ValueWeights: ValueWeights{
			Risk: 0.30, Uncertainty: 0.25, Novelty: 0.15,
			Disagreement: 0.15, FleetImpact: 0.10, PathRisk: 0.05,
		},
		CostWeights:      CostWeights{Cloud: 0.25, Latency: 0.25, Network: 0.30, Privacy: 0.20},
		ScoreWeights:     ScoreWeights{Value: 0.55, Gain: 0.35, Cost: 0.30},
		Threshold:        ThresholdConfig{Base: 0.55, Queue: 0.20, Budget: 0.15, Network: 0.25, Audit: 0.10},
		NetworkWeights:   NetworkWeights{RTT: 0.30, Fail: 0.50, Health: 0.20},
		Temperature:      1.2,
		UncertaintyEta:   0.75,
		TemplateTau:      20,
		MaxFleetDevices:  20,
		TotalLabels:      8,
		UCBAlpha:         0.15,
		BetaPriorAlpha:   1,
		BetaPriorBeta:    3,
		EstimatedTokens:  900,
		TargetLatencyMS:  5000,
		FailRateLimit:    0.20,
		ExpectedGainBias: 0.05,
		PathRisk: map[string]float64{
			"RULE":     0.05,
			"CACHE":    0.10,
			"MODEL":    0.45,
			"EDGE_LLM": 0.65,
			"FALLBACK": 1.00,
		},
	}
}
