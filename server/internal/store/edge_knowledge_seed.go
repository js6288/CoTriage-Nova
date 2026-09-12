package store

import (
	"context"
	"strings"

	"github.com/pangjian-pj/cotriage-dev/server/internal/model"
	"gorm.io/gorm"
)

const builtinRuleSetName = "内置常用规则集"

func (s *EdgeKnowledgeStore) SeedBuiltinRules(ctx context.Context) (inserted int, activated bool, err error) {
	for _, req := range builtinRuleRequests() {
		var existing EdgeRuleModel
		err := s.DB.WithContext(ctx).First(&existing, "name = ? AND deleted_at IS NULL", req.Name).Error
		if err == nil {
			continue
		}
		if err != gorm.ErrRecordNotFound {
			return inserted, activated, err
		}
		if _, err := s.CreateRule(ctx, req); err != nil {
			return inserted, activated, err
		}
		inserted++
	}

	var activeCount int64
	if err := s.DB.WithContext(ctx).Model(&EdgeRuleSetModel{}).
		Where("status = ?", string(model.SetStatusActive)).
		Count(&activeCount).Error; err != nil {
		return inserted, activated, err
	}
	if activeCount > 0 {
		return inserted, false, nil
	}

	ids, err := s.builtinRuleIDs(ctx)
	if err != nil {
		return inserted, activated, err
	}
	if len(ids) == 0 {
		return inserted, false, nil
	}
	set, err := s.CreateRuleSet(ctx, model.CreateSetRequest{
		Name:        builtinRuleSetName,
		Description: "系统预置的常见日志分诊规则，可直接启用作为 RULE 路径默认规则集。",
		ItemIDs:     ids,
	})
	if err != nil {
		return inserted, activated, err
	}
	if _, err := s.ActivateRuleSet(ctx, set.ID); err != nil {
		return inserted, activated, err
	}
	return inserted, true, nil
}

func (s *EdgeKnowledgeStore) builtinRuleIDs(ctx context.Context) ([]int64, error) {
	names := make([]string, 0, len(builtinRuleRequests()))
	for _, req := range builtinRuleRequests() {
		names = append(names, req.Name)
	}
	var rows []EdgeRuleModel
	if err := s.DB.WithContext(ctx).
		Select("id").
		Where("name IN ? AND deleted_at IS NULL AND enabled = ?", names, true).
		Order("priority DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids, nil
}

func builtinRuleRequests() []model.UpsertEdgeRuleRequest {
	status := string(model.KnowledgeStatusPublished)
	return []model.UpsertEdgeRuleRequest{
		builtinRule(status, "内置-OOM/内存不足", "regex", `out of memory|oom[- ]?killed|cannot allocate memory|memory allocation failed`, nil, "INFRA_ERROR", 95, 0.99, []string{"out of memory", "oom killed"}, "CHECK_MEMORY", "检查容器/进程内存限制、节点内存水位和近期内存突增进程。", 1000),
		builtinRule(status, "内置-磁盘空间不足", "regex", `no space left on device|disk (?:is )?full|insufficient disk space|inode.*(full|exhausted)`, nil, "DISK_ERROR", 94, 0.99, []string{"disk full"}, "CHECK_DISK_SPACE", "检查磁盘空间、inode、日志滚动策略和临时文件清理情况。", 990),
		builtinRule(status, "内置-网络连接失败", "regex", `connection refused|connection reset by peer|network is unreachable|no route to host|connection timed out`, nil, "NET_ERROR", 82, 0.95, []string{"network connection failure"}, "CHECK_NETWORK_AND_SERVICE", "检查目标服务监听状态、Service/Endpoint、网络策略和跨节点连通性。", 900),
		builtinRule(status, "内置-请求超时", "regex", `context deadline exceeded|i/o timeout|request timed out|timeout awaiting headers|gateway timeout`, nil, "NET_ERROR", 76, 0.9, []string{"timeout"}, "CHECK_TIMEOUT_AND_LATENCY", "检查依赖服务延迟、网络抖动、超时配置和下游负载。", 860),
		builtinRule(status, "内置-认证鉴权失败", "regex", `authentication failed|access denied|unauthorized|permission denied|forbidden|invalid token|token expired`, nil, "SECURITY", 78, 0.92, []string{"auth failure"}, "CHECK_AUTH", "检查访问凭据、Token 有效期、账号权限、RBAC 和异常登录来源。", 840),
	}
}

func builtinRule(status, name, matchType, pattern string, keywords []string, edgePred string, severity int, confidence float64, evidence []string, adviceCode, advice string, priority int) model.UpsertEdgeRuleRequest {
	return model.UpsertEdgeRuleRequest{
		Name:        strings.TrimSpace(name),
		Description: "系统内置规则，可作为初始分诊能力直接启用。",
		MatchType:   matchType,
		Pattern:     pattern,
		Keywords:    keywords,
		TargetField: "template_text",
		EdgePred:    edgePred,
		Severity:    severity,
		Confidence:  confidence,
		Evidence:    evidence,
		AdviceCode:  adviceCode,
		Advice:      advice,
		Priority:    priority,
		Enabled:     true,
		Status:      status,
	}
}
