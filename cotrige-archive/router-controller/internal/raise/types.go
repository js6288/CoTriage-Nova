package raise

import "github.com/pangjian-pj/cotriage-dev/router-controller/internal/event"

const (
	ActionLocal = "LOCAL"
	ActionCloud = "CLOUD"
	ActionDefer = "DEFER"

	ReasonDenyCloudPolicy    = "DENY_CLOUD_POLICY"
	ReasonForceLocalPolicy   = "FORCE_LOCAL_POLICY"
	ReasonForceCloudSendable = "FORCE_CLOUD_SENDABLE"
	ReasonForceCloudDefer    = "FORCE_CLOUD_DEFER"
	ReasonLowScore           = "LOW_SCORE"
	ReasonHighScoreSendable  = "HIGH_SCORE_SENDABLE"
	ReasonHighScoreDefer     = "HIGH_SCORE_DEFER"
)

type ValueWeights struct {
	Risk         float64
	Uncertainty  float64
	Novelty      float64
	Disagreement float64
	FleetImpact  float64
	PathRisk     float64
}

type CostWeights struct {
	Cloud   float64
	Latency float64
	Network float64
	Privacy float64
}

type ScoreWeights struct {
	Value float64
	Gain  float64
	Cost  float64
}

type ThresholdConfig struct {
	Base    float64
	Queue   float64
	Budget  float64
	Network float64
	Audit   float64
}

type NetworkWeights struct {
	RTT    float64
	Fail   float64
	Health float64
}

type Config struct {
	PolicyRef    string
	RaiseVersion string

	ValueWeights     ValueWeights
	CostWeights      CostWeights
	ScoreWeights     ScoreWeights
	Threshold        ThresholdConfig
	NetworkWeights   NetworkWeights
	Temperature      float64
	UncertaintyEta   float64
	TemplateTau      float64
	MaxFleetDevices  int
	TotalLabels      int
	UCBAlpha         float64
	BetaPriorAlpha   float64
	BetaPriorBeta    float64
	EstimatedTokens  float64
	TargetLatencyMS  float64
	FailRateLimit    float64
	PathRisk         map[string]float64
	ExpectedGainBias float64
}

type Stats struct {
	TemplateFrequency         int
	HistoricalCloudCount      int
	HistoricalCorrectionCount int
	AffectedDevices           int
	ConformalSetSize          int
	UCBUncertainty            float64
}

type SystemState struct {
	CloudHealth           string
	RTTMS                 float64
	FailRate              float64
	RateLimitRemaining    int
	MinuteBudgetRemaining float64
	HourBudgetRemaining   float64
	DayBudgetRemaining    float64
	CloudQueueLag         int
	OutboxLag             int
	QueueMax              int
	RecentCloudRatio      float64
	MinCloudRatioForAudit float64
	EstimatedLatencyMS    float64
	PrivacyCost           float64
}

type Policy struct {
	DenyCloudEdgePreds  map[string]bool
	ForceLocalEdgePreds map[string]bool
	ForceCloudEdgePreds map[string]bool
	DenyCloudSources    map[string]bool
	ForceCloudTemplates map[string]bool
}

type Features struct {
	Risk                   float64 `json:"risk"`
	CalibratedUncertainty  float64 `json:"calibrated_uncertainty"`
	ConformalSetSize       int     `json:"conformal_set_size"`
	Novelty                float64 `json:"novelty"`
	HistoricalDisagreement float64 `json:"historical_disagreement"`
	FleetImpact            float64 `json:"fleet_impact"`
	PathRisk               float64 `json:"path_risk"`
	ExpectedGain           float64 `json:"expected_gain"`
	UCBBonus               float64 `json:"ucb_bonus"`
	CloudValue             float64 `json:"cloud_value"`
}

type Costs struct {
	Cloud   float64 `json:"cloud_cost"`
	Latency float64 `json:"latency_cost"`
	Network float64 `json:"network_cost"`
	Privacy float64 `json:"privacy_cost"`
	Total   float64 `json:"total_cost"`
}

type Pressures struct {
	Queue   float64 `json:"queue_pressure"`
	Budget  float64 `json:"budget_pressure"`
	Network float64 `json:"network_pressure"`
	Audit   float64 `json:"audit_pressure"`
}

type Decision struct {
	EventID      string
	Action       string
	Reason       string
	PolicyRef    string
	Score        float64
	Threshold    float64
	Features     Features
	Costs        Costs
	Pressures    Pressures
	CloudHealth  string
	RaiseVersion string
	Sendable     bool
}

type Router interface {
	Decide(in event.InferredEvent, state SystemState, stats Stats, policy Policy) Decision
}
