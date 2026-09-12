package infer

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

type compiledRule struct {
	rule    Rule
	pattern *regexp.Regexp
}

type RuleEngine struct {
	version string
	rules   []compiledRule
}

func LoadRules(path string) (*RuleEngine, error) {
	rules := defaultRules()
	if strings.TrimSpace(path) != "" {
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read rules file: %w", err)
		}
		if err := json.Unmarshal(body, &rules); err != nil {
			return nil, fmt.Errorf("decode rules file: %w", err)
		}
	}
	return NewRuleEngine(DefaultRuleVers, rules)
}

func NewRuleEngine(version string, rules []Rule) (*RuleEngine, error) {
	sort.SliceStable(rules, func(i, j int) bool { return rules[i].Priority > rules[j].Priority })
	engine := &RuleEngine{version: version}
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if _, ok := validPredictions[rule.EdgePred]; !ok {
			return nil, fmt.Errorf("rule %q has invalid edge_pred %q", rule.ID, rule.EdgePred)
		}
		if rule.Severity < 0 || rule.Severity > 100 || rule.Confidence < 0 || rule.Confidence > 1 {
			return nil, fmt.Errorf("rule %q has out-of-range score", rule.ID)
		}
		pattern, err := regexp.Compile(`(?i)` + rule.Pattern)
		if err != nil {
			return nil, fmt.Errorf("compile rule %q: %w", rule.ID, err)
		}
		engine.rules = append(engine.rules, compiledRule{rule: rule, pattern: pattern})
	}
	return engine, nil
}

func (engine *RuleEngine) Match(text string) (TemplateResult, bool) {
	for _, item := range engine.rules {
		if item.pattern.MatchString(text) {
			result := TemplateResult{
				EdgePred: item.rule.EdgePred, Severity: item.rule.Severity,
				Confidence: item.rule.Confidence, Evidence: item.rule.Evidence,
				AdviceCode: item.rule.AdviceCode, Advice: item.rule.Advice,
			}
			if len(result.Evidence) == 0 {
				result.Evidence = []string{item.rule.ID}
			}
			return result, true
		}
	}
	return TemplateResult{}, false
}

func (engine *RuleEngine) Version() string { return engine.version }

func defaultRules() []Rule {
	return []Rule{
		{ID: "oom", Pattern: `out of memory|oom[- ]?killed|cannot allocate memory`, EdgePred: PredInfraError, Severity: 95, Confidence: 0.99, Evidence: []string{"out of memory"}, AdviceCode: "CHECK_MEMORY", Advice: "检查内存使用、限制配置和进程状态", Priority: 100, Enabled: true},
		{ID: "disk-full", Pattern: `no space left on device|disk (?:is )?full`, EdgePred: PredDiskError, Severity: 92, Confidence: 0.99, Evidence: []string{"no space left on device"}, AdviceCode: "CHECK_DISK_SPACE", Advice: "检查磁盘空间、inode和日志清理策略", Priority: 100, Enabled: true},
		{ID: "network-refused", Pattern: `connection refused|network is unreachable|no route to host`, EdgePred: PredNetError, Severity: 78, Confidence: 0.95, Evidence: []string{"connection failure"}, AdviceCode: "CHECK_NETWORK_AND_SERVICE", Advice: "检查目标服务状态、端口监听和网络策略", Priority: 80, Enabled: true},
		{ID: "auth-failed", Pattern: `authentication failed|access denied|unauthorized`, EdgePred: PredSecurity, Severity: 75, Confidence: 0.92, Evidence: []string{"authentication failure"}, AdviceCode: "CHECK_AUTH", Advice: "检查访问凭据、账号权限和异常登录来源", Priority: 70, Enabled: true},
	}
}
