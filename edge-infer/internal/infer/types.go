package infer

const (
	PredNormal      = "NORMAL"
	PredAppError    = "APP_ERROR"
	PredNetError    = "NET_ERROR"
	PredDiskError   = "DISK_ERROR"
	PredInfraError  = "INFRA_ERROR"
	PredSecurity    = "SECURITY"
	PredAbnormal    = "ABNORMAL"
	PredUnknown     = "UNKNOWN"
	PathRule        = "RULE"
	PathCache       = "CACHE"
	PathModel       = "MODEL"
	PathEdgeLLM     = "EDGE_LLM"
	PathFallback    = "FALLBACK"
	DefaultRuleVers = "edge-rules-v1"
)

var validPredictions = map[string]struct{}{
	PredNormal: {}, PredAppError: {}, PredNetError: {}, PredDiskError: {},
	PredInfraError: {}, PredSecurity: {}, PredAbnormal: {}, PredUnknown: {},
}

type Rule struct {
	ID         string   `json:"id"`
	Pattern    string   `json:"pattern"`
	EdgePred   string   `json:"edge_pred"`
	Severity   int      `json:"severity"`
	Confidence float64  `json:"confidence"`
	Evidence   []string `json:"evidence,omitempty"`
	AdviceCode string   `json:"advice_code,omitempty"`
	Advice     string   `json:"advice,omitempty"`
	Priority   int      `json:"priority"`
	Enabled    bool     `json:"enabled"`
}

type TemplateResult struct {
	TemplateID string   `json:"template_id,omitempty"`
	Template   string   `json:"template_text,omitempty"`
	EdgePred   string   `json:"edge_pred"`
	Severity   int      `json:"severity"`
	Confidence float64  `json:"confidence"`
	Evidence   []string `json:"evidence,omitempty"`
	AdviceCode string   `json:"advice_code,omitempty"`
	Advice     string   `json:"advice,omitempty"`
}
