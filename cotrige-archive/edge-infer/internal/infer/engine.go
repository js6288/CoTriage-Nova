package infer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pangjian-pj/cotriage-dev/edge-infer/internal/event"
	"github.com/redis/go-redis/v9"
)

type Engine struct {
	mu          sync.RWMutex
	redis       *redis.Client
	rules       *RuleEngine
	templates   map[string]TemplateResult
	classifier  Classifier
	edgeLLM     EdgeLLM
	cacheTTL    time.Duration
	maxLogBytes int
}

func NewEngine(rdb *redis.Client, rules *RuleEngine, templatesFile string, cacheTTL time.Duration, maxLogBytes int, classifier Classifier, edgeLLM EdgeLLM) (*Engine, error) {
	templates, err := loadTemplates(templatesFile)
	if err != nil {
		return nil, err
	}
	return &Engine{redis: rdb, rules: rules, templates: templates, classifier: classifier, edgeLLM: edgeLLM, cacheTTL: cacheTTL, maxLogBytes: maxLogBytes}, nil
}

func (engine *Engine) Infer(ctx context.Context, raw event.RawEvent) (event.InferredEvent, error) {
	start := time.Now()
	if strings.TrimSpace(raw.EventID) == "" {
		return event.InferredEvent{}, fmt.Errorf("event_id is empty")
	}
	if strings.TrimSpace(raw.Log.Raw) == "" {
		return event.InferredEvent{}, fmt.Errorf("log.raw is empty")
	}

	normalizeStart := time.Now()
	templateText := Normalize(raw.Log.Raw, engine.maxLogBytes)
	templateID := TemplateID(templateText)
	rules, templates := engine.knowledgeSnapshot()
	classifier, edgeLLM := engine.modelSnapshot()
	ruleVersion := rules.Version()
	result := baseResult(raw, templateID, templateText, ruleVersion)
	result.StageLatency.Normalize = time.Since(normalizeStart).Milliseconds()

	if matched, ok := rules.Match(templateText); ok {
		applyResult(&result, matched, PathRule)
		result.Finish(start)
		engine.cacheBestEffort(ctx, &result, templateID, matched, ruleVersion)
		return result, nil
	}

	cacheStart := time.Now()
	if cached, ok := templates[templateID]; ok {
		applyResult(&result, cached, PathCache)
		result.StageLatency.Cache = time.Since(cacheStart).Milliseconds()
		result.Finish(start)
		return result, nil
	}
	if cached, ok, err := engine.loadCache(ctx, templateID, ruleVersion); err != nil {
		result.Evidence = append(result.Evidence, fmt.Sprintf("template_cache_error=%v", err))
	} else if ok {
		applyResult(&result, cached, PathCache)
		result.StageLatency.Cache = time.Since(cacheStart).Milliseconds()
		result.Finish(start)
		return result, nil
	}
	result.StageLatency.Cache = time.Since(cacheStart).Milliseconds()

	if classifier != nil {
		modelStart := time.Now()
		classified, err := classifier.Predict(ctx, ClassifierInput{
			EventID: raw.EventID, TemplateID: templateID, TemplateText: templateText,
		})
		result.StageLatency.Model = time.Since(modelStart).Milliseconds()
		if err != nil {
			applyFallback(&result, fmt.Sprintf("classifier_error=%v", err))
			result.Finish(start)
			return result, nil
		}
		modelResult := resultFromClassifier(classified)
		applyResult(&result, modelResult, PathModel)
		result.ModelVersion = classified.ModelVersion
		if edgeLLM != nil && modelResult.EdgePred == PredAbnormal {
			llmStart := time.Now()
			diagnosis, err := edgeLLM.Diagnose(ctx, LLMInput{
				EventID: raw.EventID, Equipment: raw.Equipment, RawLog: result.RawLog,
				TemplateID: templateID, TemplateText: templateText, Classifier: classified,
			})
			result.StageLatency.EdgeLLM = time.Since(llmStart).Milliseconds()
			if err == nil {
				llmResult := resultFromLLM(diagnosis)
				applyResult(&result, llmResult, PathEdgeLLM)
				result.ModelVersion = diagnosis.ModelVersion
				result.Finish(start)
				engine.cacheBestEffort(ctx, &result, templateID, llmResult, ruleVersion)
				return result, nil
			}
			result.Evidence = append(result.Evidence, fmt.Sprintf("edge_llm_error=%v", err))
		}
		result.Finish(start)
		engine.cacheBestEffort(ctx, &result, templateID, modelResult, ruleVersion)
		return result, nil
	}

	applyFallback(&result)
	result.Finish(start)
	return result, nil
}

func (engine *Engine) UpdateKnowledge(rules *RuleEngine, templates map[string]TemplateResult) {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if rules != nil {
		engine.rules = rules
	}
	if templates != nil {
		engine.templates = templates
	}
}

func (engine *Engine) UpdateModels(classifier Classifier, edgeLLM EdgeLLM) {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.classifier = classifier
	engine.edgeLLM = edgeLLM
}

func (engine *Engine) knowledgeSnapshot() (*RuleEngine, map[string]TemplateResult) {
	engine.mu.RLock()
	defer engine.mu.RUnlock()
	return engine.rules, engine.templates
}

func (engine *Engine) modelSnapshot() (Classifier, EdgeLLM) {
	engine.mu.RLock()
	defer engine.mu.RUnlock()
	return engine.classifier, engine.edgeLLM
}

func (engine *Engine) cacheBestEffort(ctx context.Context, result *event.InferredEvent, templateID string, value TemplateResult, ruleVersion string) {
	if err := engine.cache(ctx, templateID, value, ruleVersion); err != nil {
		result.Evidence = append(result.Evidence, fmt.Sprintf("template_cache_write_error=%v", err))
	}
}

func baseResult(raw event.RawEvent, templateID, templateText, ruleVersion string) event.InferredEvent {
	return event.InferredEvent{
		EventID: raw.EventID, Source: raw.Source, Equipment: raw.Equipment,
		EventTS: raw.TS, RawLog: truncateRawLog(raw.Log.Raw),
		TemplateID: templateID, TemplateText: templateText,
		RuleVersion: ruleVersion, Evidence: []string{},
	}
}

func truncateRawLog(value string) string {
	value = strings.TrimSpace(value)
	const maxRawLogRunes = 2048
	runes := []rune(value)
	if len(runes) <= maxRawLogRunes {
		return value
	}
	return string(runes[:maxRawLogRunes]) + "...[truncated]"
}

func applyResult(output *event.InferredEvent, input TemplateResult, path string) {
	output.EdgePred = input.EdgePred
	output.Severity = input.Severity
	output.Confidence = input.Confidence
	output.Uncertainty = 1 - input.Confidence
	output.Evidence = input.Evidence
	if output.Evidence == nil {
		output.Evidence = []string{}
	}
	output.AdviceCode = input.AdviceCode
	output.Advice = input.Advice
	output.InferencePath = path
}

func applyFallback(output *event.InferredEvent, evidence ...string) {
	output.EdgePred = PredUnknown
	output.Severity = 50
	output.Confidence = 0
	output.Uncertainty = 1
	output.InferencePath = PathFallback
	output.Evidence = evidence
	if output.Evidence == nil {
		output.Evidence = []string{}
	}
}

func (engine *Engine) cache(ctx context.Context, templateID string, result TemplateResult, ruleVersion string) error {
	if result.Confidence < 0.85 || result.EdgePred == PredUnknown {
		return nil
	}
	body, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return engine.redis.Set(ctx, engine.cacheKey(templateID, ruleVersion), body, engine.cacheTTL).Err()
}

func (engine *Engine) loadCache(ctx context.Context, templateID, ruleVersion string) (TemplateResult, bool, error) {
	body, err := engine.redis.Get(ctx, engine.cacheKey(templateID, ruleVersion)).Bytes()
	if err == redis.Nil {
		return TemplateResult{}, false, nil
	}
	if err != nil {
		return TemplateResult{}, false, fmt.Errorf("read template cache: %w", err)
	}
	var result TemplateResult
	if err := json.Unmarshal(body, &result); err != nil {
		return TemplateResult{}, false, fmt.Errorf("decode template cache: %w", err)
	}
	return result, true, nil
}

func (engine *Engine) cacheKey(templateID, ruleVersion string) string {
	return "cotriage:edge-infer:template:" + templateID + ":none:" + ruleVersion
}

func loadTemplates(path string) (map[string]TemplateResult, error) {
	results := make(map[string]TemplateResult)
	if strings.TrimSpace(path) == "" {
		return results, nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read templates file: %w", err)
	}
	var templates []TemplateResult
	if err := json.Unmarshal(body, &templates); err != nil {
		return nil, fmt.Errorf("decode templates file: %w", err)
	}
	for _, item := range templates {
		id := strings.TrimSpace(item.TemplateID)
		if id == "" && item.Template != "" {
			id = TemplateID(item.Template)
		}
		if id == "" {
			return nil, fmt.Errorf("template entry has neither template_id nor template_text")
		}
		if _, ok := validPredictions[item.EdgePred]; !ok {
			return nil, fmt.Errorf("template %q has invalid edge_pred %q", id, item.EdgePred)
		}
		results[id] = item
	}
	return results, nil
}
