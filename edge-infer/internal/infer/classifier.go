package infer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Classifier interface {
	Predict(ctx context.Context, input ClassifierInput) (ClassifierResult, error)
}

type ClassifierInput struct {
	EventID      string `json:"event_id,omitempty"`
	TemplateID   string `json:"template_id"`
	TemplateText string `json:"template_text"`
}

type ClassifierResult struct {
	Label            string             `json:"label"`
	LabelID          int                `json:"label_id"`
	AlertProbability float64            `json:"alert_probability"`
	Threshold        float64            `json:"threshold"`
	Probabilities    map[string]float64 `json:"probabilities,omitempty"`
	ModelVersion     string             `json:"model_version,omitempty"`
}

type HTTPClassifier struct {
	url    string
	client *http.Client
}

func NewHTTPClassifier(url string, timeout time.Duration) (*HTTPClassifier, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return nil, nil
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("classifier timeout must be positive")
	}
	return &HTTPClassifier{
		url:    url,
		client: &http.Client{Timeout: timeout},
	}, nil
}

func (classifier *HTTPClassifier) Predict(ctx context.Context, input ClassifierInput) (ClassifierResult, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return ClassifierResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, classifier.url, bytes.NewReader(body))
	if err != nil {
		return ClassifierResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := classifier.client.Do(req)
	if err != nil {
		return ClassifierResult{}, fmt.Errorf("call classifier: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ClassifierResult{}, fmt.Errorf("classifier status %s", resp.Status)
	}
	var result ClassifierResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ClassifierResult{}, fmt.Errorf("decode classifier response: %w", err)
	}
	result.Label = strings.TrimSpace(result.Label)
	if result.Label != "ALERT" && result.Label != "NON_ALERT" {
		return ClassifierResult{}, fmt.Errorf("classifier returned invalid label %q", result.Label)
	}
	if result.AlertProbability < 0 || result.AlertProbability > 1 {
		return ClassifierResult{}, fmt.Errorf("classifier returned invalid alert_probability %v", result.AlertProbability)
	}
	return result, nil
}

func resultFromClassifier(input ClassifierResult) TemplateResult {
	confidence := input.AlertProbability
	prediction := PredAbnormal
	severity := 75
	adviceCode := "CHECK_ALERT_CONTEXT"
	advice := "轻量分类模型判断该日志模板疑似异常，建议进入边缘 LLM 或云端进一步诊断。"
	if input.Label == "NON_ALERT" {
		confidence = 1 - input.AlertProbability
		prediction = PredNormal
		severity = 10
		adviceCode = "NO_ACTION"
		advice = "轻量分类模型判断该日志模板为正常。"
	}
	if confidence < 0 {
		confidence = 0
	}
	if confidence > 1 {
		confidence = 1
	}
	evidence := []string{
		fmt.Sprintf("classifier_label=%s", input.Label),
		fmt.Sprintf("alert_probability=%.6f", input.AlertProbability),
		fmt.Sprintf("threshold=%.6f", input.Threshold),
	}
	return TemplateResult{
		EdgePred: prediction, Severity: severity, Confidence: confidence,
		Evidence: evidence, AdviceCode: adviceCode, Advice: advice,
	}
}
