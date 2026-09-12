package infer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type EdgeLLM interface {
	Diagnose(ctx context.Context, input LLMInput) (LLMResult, error)
}

type LLMInput struct {
	EventID      string           `json:"event_id,omitempty"`
	Equipment    string           `json:"equipment,omitempty"`
	RawLog       string           `json:"raw_log"`
	TemplateID   string           `json:"template_id"`
	TemplateText string           `json:"template_text"`
	Classifier   ClassifierResult `json:"classifier"`
}

type LLMResult struct {
	EdgePred     string   `json:"edge_pred"`
	Severity     int      `json:"severity"`
	Confidence   float64  `json:"confidence"`
	Evidence     []string `json:"evidence,omitempty"`
	AdviceCode   string   `json:"advice_code,omitempty"`
	Advice       string   `json:"advice,omitempty"`
	ModelVersion string   `json:"model_version,omitempty"`
}

type OpenAIChatLLM struct {
	url         string
	model       string
	maxTokens   int
	temperature float64
	topP        float64
	seed        *int
	client      *http.Client
}

func NewOpenAIChatLLM(url string, timeout time.Duration) (*OpenAIChatLLM, error) {
	return NewOpenAIChatLLMConfig(url, "qwen1.5-1.8b-chat", timeout, 384, 0.1, 1, nil)
}

func NewOpenAIChatLLMConfig(url, model string, timeout time.Duration, maxTokens int, temperature, topP float64, seed *int) (*OpenAIChatLLM, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return nil, nil
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = "default"
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("llm timeout must be positive")
	}
	if maxTokens <= 0 {
		maxTokens = 384
	}
	return &OpenAIChatLLM{
		url:         url,
		model:       model,
		maxTokens:   maxTokens,
		temperature: temperature,
		topP:        topP,
		seed:        seed,
		client:      &http.Client{Timeout: timeout},
	}, nil
}

func (llm *OpenAIChatLLM) Diagnose(ctx context.Context, input LLMInput) (LLMResult, error) {
	reqBody := map[string]any{
		"model": llm.model,
		"messages": []map[string]string{
			{"role": "system", "content": llmSystemPrompt()},
			{"role": "user", "content": llmUserPrompt(input)},
		},
		"temperature": llm.temperature,
		"top_p":       llm.topP,
		"max_tokens":  llm.maxTokens,
		"stream":      false,
	}
	if llm.seed != nil {
		reqBody["seed"] = *llm.seed
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return LLMResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, llm.url, bytes.NewReader(body))
	if err != nil {
		return LLMResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := llm.client.Do(req)
	if err != nil {
		return LLMResult{}, fmt.Errorf("call edge llm: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return LLMResult{}, fmt.Errorf("edge llm status %s", resp.Status)
	}
	var completion chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&completion); err != nil {
		return LLMResult{}, fmt.Errorf("decode edge llm response: %w", err)
	}
	if len(completion.Choices) == 0 {
		return LLMResult{}, fmt.Errorf("edge llm returned no choices")
	}
	content := completion.Choices[0].Message.Content
	result, err := parseLLMJSON(content)
	if err != nil {
		return LLMResult{}, err
	}
	if result.ModelVersion == "" {
		result.ModelVersion = llm.model
	}
	return result, nil
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func llmSystemPrompt() string {
	return strings.Join([]string{
		"你是 CoTriage 的边缘日志故障诊断助手。",
		"你只能输出一个单行 JSON 对象，不要输出 Markdown、解释或代码块。",
		"edge_pred 只能是 APP_ERROR、NET_ERROR、DISK_ERROR、INFRA_ERROR、SECURITY、UNKNOWN 之一。",
		"severity 必须是 0 到 100 的整数。",
		"confidence 必须是 0 到 1 的数字。",
		"evidence 最多 2 条，每条不超过 20 个字符。",
		"advice 不超过 30 个汉字。",
		"输出必须形如 {\"edge_pred\":\"NET_ERROR\",\"severity\":78,\"confidence\":0.72,\"evidence\":[\"connection refused\"],\"advice_code\":\"CHECK_NETWORK\",\"advice\":\"检查服务端口\"}",
	}, "\n")
}

func llmUserPrompt(input LLMInput) string {
	payload := map[string]any{
		"equipment":     input.Equipment,
		"raw_log":       input.RawLog,
		"template_text": input.TemplateText,
		"classifier": map[string]any{
			"label":             input.Classifier.Label,
			"alert_probability": input.Classifier.AlertProbability,
			"threshold":         input.Classifier.Threshold,
		},
	}
	body, _ := json.Marshal(payload)
	return "请基于以下 JSON 诊断日志，并只返回一个紧凑 JSON 对象：\n" + string(body)
}

func parseLLMJSON(content string) (LLMResult, error) {
	content = strings.TrimSpace(content)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 {
		return LLMResult{}, fmt.Errorf("edge llm response does not contain JSON object")
	}
	if end < start {
		if repaired, repairErr := parsePartialLLMJSON(content[start:]); repairErr == nil {
			return repaired, nil
		}
		return LLMResult{}, fmt.Errorf("edge llm response contains incomplete JSON; content_preview=%q", preview(content, 240))
	}
	var result LLMResult
	if err := json.Unmarshal([]byte(content[start:end+1]), &result); err != nil {
		if repaired, repairErr := parsePartialLLMJSON(content[start:]); repairErr == nil {
			return repaired, nil
		}
		return LLMResult{}, fmt.Errorf("decode edge llm JSON: %w; content_preview=%q", err, preview(content, 240))
	}
	if err := validateLLMResult(&result); err != nil {
		return LLMResult{}, err
	}
	return result, nil
}

func validateLLMResult(result *LLMResult) error {
	result.EdgePred = strings.TrimSpace(result.EdgePred)
	if _, ok := validLLMPredictions[result.EdgePred]; !ok {
		return fmt.Errorf("edge llm returned invalid edge_pred %q", result.EdgePred)
	}
	if result.Severity < 0 || result.Severity > 100 {
		return fmt.Errorf("edge llm returned invalid severity %d", result.Severity)
	}
	if result.Confidence < 0 || result.Confidence > 1 {
		return fmt.Errorf("edge llm returned invalid confidence %v", result.Confidence)
	}
	if len(result.Evidence) > 3 {
		result.Evidence = result.Evidence[:3]
	}
	if result.Evidence == nil {
		result.Evidence = []string{}
	}
	return nil
}

func parsePartialLLMJSON(content string) (LLMResult, error) {
	result := LLMResult{
		EdgePred:   regexString(content, `"edge_pred"\s*:\s*"([^"]+)"`),
		AdviceCode: regexString(content, `"advice_code"\s*:\s*"([^"]*)"`),
		Advice:     regexString(content, `"advice"\s*:\s*"([^"]*)"`),
	}
	severity, ok := regexInt(content, `"severity"\s*:\s*([0-9]+)`)
	if !ok {
		return LLMResult{}, fmt.Errorf("partial edge llm JSON missing severity")
	}
	result.Severity = severity
	confidence, ok := regexFloat(content, `"confidence"\s*:\s*([0-9]+(?:\.[0-9]+)?)`)
	if !ok {
		return LLMResult{}, fmt.Errorf("partial edge llm JSON missing confidence")
	}
	result.Confidence = confidence
	result.Evidence = regexStringList(content, `"evidence"\s*:\s*\[([^\]]*)`)
	if len(result.Evidence) == 0 {
		result.Evidence = []string{"partial_llm_json"}
	}
	if err := validateLLMResult(&result); err != nil {
		return LLMResult{}, err
	}
	result.Evidence = append(result.Evidence, "llm_json_repaired")
	return result, nil
}

func regexString(content, pattern string) string {
	match := regexp.MustCompile(pattern).FindStringSubmatch(content)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func regexInt(content, pattern string) (int, bool) {
	match := regexp.MustCompile(pattern).FindStringSubmatch(content)
	if len(match) < 2 {
		return 0, false
	}
	value, err := strconv.Atoi(match[1])
	return value, err == nil
}

func regexFloat(content, pattern string) (float64, bool) {
	match := regexp.MustCompile(pattern).FindStringSubmatch(content)
	if len(match) < 2 {
		return 0, false
	}
	value, err := strconv.ParseFloat(match[1], 64)
	return value, err == nil
}

func regexStringList(content, pattern string) []string {
	match := regexp.MustCompile(pattern).FindStringSubmatch(content)
	if len(match) < 2 {
		return nil
	}
	itemMatches := regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(match[1], -1)
	items := make([]string, 0, len(itemMatches))
	for _, item := range itemMatches {
		if len(item) >= 2 {
			items = append(items, strings.TrimSpace(item[1]))
		}
		if len(items) >= 3 {
			break
		}
	}
	return items
}

func preview(value string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes]) + "...[truncated]"
}

var validLLMPredictions = map[string]struct{}{
	PredAppError: {}, PredNetError: {}, PredDiskError: {},
	PredInfraError: {}, PredSecurity: {}, PredUnknown: {},
}

func resultFromLLM(input LLMResult) TemplateResult {
	return TemplateResult{
		EdgePred: input.EdgePred, Severity: input.Severity, Confidence: input.Confidence,
		Evidence: input.Evidence, AdviceCode: input.AdviceCode, Advice: input.Advice,
	}
}
