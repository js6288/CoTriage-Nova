package infer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type remoteEdgeConfig struct {
	Classifier *remoteClassifier `json:"classifier"`
	Provider   *remoteProvider   `json:"active_provider"`
	Rules      struct {
		Version int64        `json:"version"`
		Etag    string       `json:"etag"`
		Items   []remoteRule `json:"items"`
	} `json:"rules"`
	Templates struct {
		Version int64            `json:"version"`
		Etag    string           `json:"etag"`
		Items   []remoteTemplate `json:"items"`
	} `json:"templates"`
}

type remoteClassifier struct {
	Endpoint  string  `json:"endpoint"`
	Model     string  `json:"model"`
	TimeoutMS int     `json:"timeout_ms"`
	Threshold float64 `json:"threshold"`
	IsActive  bool    `json:"is_active"`
	Status    string  `json:"status"`
}

type remoteProvider struct {
	Endpoint     string  `json:"endpoint"`
	Model        string  `json:"model"`
	TimeoutMS    int     `json:"timeout_ms"`
	MaxTokens    int     `json:"max_tokens"`
	Temperature  float64 `json:"temperature"`
	TopP         float64 `json:"top_p"`
	Seed         *int    `json:"seed"`
	IsActive     bool    `json:"is_active"`
	Status       string  `json:"status"`
	ResponsePath string  `json:"response_path"`
}

type remoteRule struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	MatchType   string   `json:"match_type"`
	Pattern     string   `json:"pattern"`
	Keywords    []string `json:"keywords"`
	TargetField string   `json:"target_field"`
	EdgePred    string   `json:"edge_pred"`
	Severity    int      `json:"severity"`
	Confidence  float64  `json:"confidence"`
	Evidence    []string `json:"evidence"`
	AdviceCode  string   `json:"advice_code"`
	Advice      string   `json:"advice"`
	Priority    int      `json:"priority"`
	Enabled     bool     `json:"enabled"`
	Status      string   `json:"status"`
}

type remoteTemplate struct {
	TemplateID   string   `json:"template_id"`
	TemplateText string   `json:"template_text"`
	EdgePred     string   `json:"edge_pred"`
	Severity     int      `json:"severity"`
	Confidence   float64  `json:"confidence"`
	Evidence     []string `json:"evidence"`
	AdviceCode   string   `json:"advice_code"`
	Advice       string   `json:"advice"`
	Status       string   `json:"status"`
}

func StartRemoteConfigSync(ctx context.Context, engine *Engine, url string, interval, timeout time.Duration) {
	if strings.TrimSpace(url) == "" {
		return
	}
	go runRemoteConfigSync(ctx, engine, strings.TrimSpace(url), interval, timeout)
}

func runRemoteConfigSync(ctx context.Context, engine *Engine, url string, interval, timeout time.Duration) {
	client := &http.Client{Timeout: timeout}
	var etag string
	if next, ok := fetchAndApplyRemoteConfig(ctx, client, engine, url, etag); ok {
		etag = next
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if next, ok := fetchAndApplyRemoteConfig(ctx, client, engine, url, etag); ok {
				etag = next
			}
		}
	}
}

func fetchAndApplyRemoteConfig(ctx context.Context, client *http.Client, engine *Engine, url, etag string) (string, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Printf("[edge-infer] invalid edge config url=%s error=%v", url, err)
		return etag, false
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[edge-infer] fetch edge config error=%v", err)
		return etag, false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return etag, false
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		log.Printf("[edge-infer] fetch edge config status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
		return etag, false
	}

	var cfg remoteEdgeConfig
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		log.Printf("[edge-infer] decode edge config error=%v", err)
		return etag, false
	}

	rules, err := rulesFromRemote(cfg.Rules.Version, cfg.Rules.Items)
	if err != nil {
		log.Printf("[edge-infer] apply remote rules error=%v", err)
		return etag, false
	}
	templates, err := templatesFromRemote(cfg.Templates.Items)
	if err != nil {
		log.Printf("[edge-infer] apply remote templates error=%v", err)
		return etag, false
	}
	classifier, err := classifierFromRemote(cfg.Classifier)
	if err != nil {
		log.Printf("[edge-infer] apply remote classifier error=%v", err)
		return etag, false
	}
	edgeLLM, err := llmFromRemote(cfg.Provider)
	if err != nil {
		log.Printf("[edge-infer] apply remote edge llm error=%v", err)
		return etag, false
	}
	engine.UpdateKnowledge(rules, templates)
	engine.UpdateModels(classifier, edgeLLM)

	next := strings.TrimSpace(resp.Header.Get("ETag"))
	if next == "" {
		next = strings.TrimSpace(cfg.Rules.Etag + cfg.Templates.Etag)
	}
	log.Printf("[edge-infer] edge config applied rules=%d templates=%d classifier=%t edge_llm=%t etag=%s", len(cfg.Rules.Items), len(cfg.Templates.Items), classifier != nil, edgeLLM != nil, next)
	return next, true
}

func classifierFromRemote(cfg *remoteClassifier) (Classifier, error) {
	if cfg == nil || !cfg.IsActive || cfg.Status != "published" {
		return nil, nil
	}
	timeout := time.Duration(cfg.TimeoutMS) * time.Millisecond
	return NewHTTPClassifier(cfg.Endpoint, timeout)
}

func llmFromRemote(cfg *remoteProvider) (EdgeLLM, error) {
	if cfg == nil || !cfg.IsActive || cfg.Status != "published" {
		return nil, nil
	}
	timeout := time.Duration(cfg.TimeoutMS) * time.Millisecond
	return NewOpenAIChatLLMConfig(cfg.Endpoint, cfg.Model, timeout, cfg.MaxTokens, cfg.Temperature, cfg.TopP, cfg.Seed)
}

func rulesFromRemote(version int64, items []remoteRule) (*RuleEngine, error) {
	rules := make([]Rule, 0, len(items))
	for _, item := range items {
		if !item.Enabled || item.Status == "disabled" || item.Status == "deleted" {
			continue
		}
		pattern, err := remoteRulePattern(item)
		if err != nil {
			return nil, err
		}
		id := strings.TrimSpace(item.Name)
		if id == "" {
			id = strconv.FormatInt(item.ID, 10)
		}
		rules = append(rules, Rule{
			ID:         id,
			Pattern:    pattern,
			EdgePred:   item.EdgePred,
			Severity:   item.Severity,
			Confidence: item.Confidence,
			Evidence:   item.Evidence,
			AdviceCode: item.AdviceCode,
			Advice:     item.Advice,
			Priority:   item.Priority,
			Enabled:    true,
		})
	}
	return NewRuleEngine(fmt.Sprintf("edge-rules-%d", version), rules)
}

func remoteRulePattern(item remoteRule) (string, error) {
	switch strings.TrimSpace(item.MatchType) {
	case "contains":
		if strings.TrimSpace(item.Pattern) == "" {
			return "", fmt.Errorf("rule %d contains pattern is empty", item.ID)
		}
		return regexp.QuoteMeta(strings.TrimSpace(item.Pattern)), nil
	case "regex":
		return strings.TrimSpace(item.Pattern), nil
	case "all_keywords":
		parts := make([]string, 0, len(item.Keywords))
		for _, keyword := range item.Keywords {
			keyword = strings.TrimSpace(keyword)
			if keyword != "" {
				parts = append(parts, "(?=.*"+regexp.QuoteMeta(keyword)+")")
			}
		}
		if len(parts) == 0 {
			return "", fmt.Errorf("rule %d all_keywords is empty", item.ID)
		}
		return strings.Join(parts, "") + ".*", nil
	case "any_keywords":
		parts := make([]string, 0, len(item.Keywords))
		for _, keyword := range item.Keywords {
			keyword = strings.TrimSpace(keyword)
			if keyword != "" {
				parts = append(parts, regexp.QuoteMeta(keyword))
			}
		}
		if len(parts) == 0 {
			return "", fmt.Errorf("rule %d any_keywords is empty", item.ID)
		}
		return "(" + strings.Join(parts, "|") + ")", nil
	default:
		return "", fmt.Errorf("rule %d has invalid match_type %q", item.ID, item.MatchType)
	}
}

func templatesFromRemote(items []remoteTemplate) (map[string]TemplateResult, error) {
	templates := make(map[string]TemplateResult, len(items))
	for _, item := range items {
		if item.Status == "disabled" || item.Status == "deleted" {
			continue
		}
		id := strings.TrimSpace(item.TemplateID)
		if id == "" {
			id = TemplateID(item.TemplateText)
		}
		if _, ok := validPredictions[item.EdgePred]; !ok {
			return nil, fmt.Errorf("template %q has invalid edge_pred %q", id, item.EdgePred)
		}
		templates[id] = TemplateResult{
			TemplateID: id,
			Template:   item.TemplateText,
			EdgePred:   item.EdgePred,
			Severity:   item.Severity,
			Confidence: item.Confidence,
			Evidence:   item.Evidence,
			AdviceCode: item.AdviceCode,
			Advice:     item.Advice,
		}
	}
	return templates, nil
}
