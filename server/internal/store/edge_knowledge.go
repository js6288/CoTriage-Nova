package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/pangjian-pj/cotriage-dev/server/internal/model"
	"gorm.io/gorm"
)

type EdgeKnowledgeStore struct {
	DB *gorm.DB
}

func NewEdgeKnowledgeStore(db *gorm.DB) *EdgeKnowledgeStore { return &EdgeKnowledgeStore{DB: db} }

type ListKnowledgeQuery struct {
	Status   string
	EdgePred string
	Q        string
	Page     int
	PageSize int
}

func (s *EdgeKnowledgeStore) CreateRule(ctx context.Context, req model.UpsertEdgeRuleRequest) (model.EdgeRule, error) {
	if err := validateRule(req); err != nil {
		return model.EdgeRule{}, err
	}
	status := req.Status
	if status == "" {
		status = string(model.KnowledgeStatusDraft)
	}
	m := EdgeRuleModel{
		Name: strings.TrimSpace(req.Name), Description: strings.TrimSpace(req.Description),
		MatchType: strings.TrimSpace(req.MatchType), Pattern: strings.TrimSpace(req.Pattern),
		KeywordsJSON: mustJSON(req.Keywords), TargetField: defaultString(req.TargetField, "template_text"),
		EdgePred: req.EdgePred, Severity: req.Severity, Confidence: req.Confidence,
		EvidenceJSON: mustJSON(req.Evidence), AdviceCode: strings.TrimSpace(req.AdviceCode),
		Advice: strings.TrimSpace(req.Advice), Priority: req.Priority, Enabled: req.Enabled, Status: status,
	}
	if err := s.DB.WithContext(ctx).Create(&m).Error; err != nil {
		return model.EdgeRule{}, err
	}
	return ruleFromModel(m), nil
}

func (s *EdgeKnowledgeStore) PatchRule(ctx context.Context, id int64, req model.UpsertEdgeRuleRequest) (model.EdgeRule, error) {
	if err := validateRule(req); err != nil {
		return model.EdgeRule{}, err
	}
	var m EdgeRuleModel
	if err := s.DB.WithContext(ctx).First(&m, "id = ? AND deleted_at IS NULL", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return model.EdgeRule{}, ErrNotFound
		}
		return model.EdgeRule{}, err
	}
	m.Name = strings.TrimSpace(req.Name)
	m.Description = strings.TrimSpace(req.Description)
	m.MatchType = strings.TrimSpace(req.MatchType)
	m.Pattern = strings.TrimSpace(req.Pattern)
	m.KeywordsJSON = mustJSON(req.Keywords)
	m.TargetField = defaultString(req.TargetField, "template_text")
	m.EdgePred = req.EdgePred
	m.Severity = req.Severity
	m.Confidence = req.Confidence
	m.EvidenceJSON = mustJSON(req.Evidence)
	m.AdviceCode = strings.TrimSpace(req.AdviceCode)
	m.Advice = strings.TrimSpace(req.Advice)
	m.Priority = req.Priority
	m.Enabled = req.Enabled
	if req.Status != "" {
		m.Status = req.Status
	}
	if err := s.DB.WithContext(ctx).Save(&m).Error; err != nil {
		return model.EdgeRule{}, err
	}
	return ruleFromModel(m), nil
}

func (s *EdgeKnowledgeStore) ListRules(ctx context.Context, q ListKnowledgeQuery) ([]model.EdgeRule, int, error) {
	page, ps := normalizePage(q.Page, q.PageSize)
	dbq := s.DB.WithContext(ctx).Model(&EdgeRuleModel{}).Where("deleted_at IS NULL")
	if q.Status != "" {
		dbq = dbq.Where("status = ?", q.Status)
	}
	if q.EdgePred != "" {
		dbq = dbq.Where("edge_pred = ?", q.EdgePred)
	}
	if strings.TrimSpace(q.Q) != "" {
		like := "%" + strings.TrimSpace(q.Q) + "%"
		dbq = dbq.Where("name LIKE ? OR pattern LIKE ? OR advice LIKE ?", like, like, like)
	}
	var total int64
	if err := dbq.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var ms []EdgeRuleModel
	if err := dbq.Order("priority ASC").Order("updated_at DESC").Limit(ps).Offset((page - 1) * ps).Find(&ms).Error; err != nil {
		return nil, 0, err
	}
	items := make([]model.EdgeRule, 0, len(ms))
	for _, m := range ms {
		items = append(items, ruleFromModel(m))
	}
	return items, safeInt(total), nil
}

func (s *EdgeKnowledgeStore) DeleteRule(ctx context.Context, id int64) error {
	now := time.Now().UTC()
	res := s.DB.WithContext(ctx).Model(&EdgeRuleModel{}).Where("id = ? AND deleted_at IS NULL", id).Updates(map[string]any{"status": string(model.KnowledgeStatusDeleted), "deleted_at": &now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *EdgeKnowledgeStore) CreateTemplate(ctx context.Context, req model.UpsertEdgeTemplateRequest) (model.EdgeTemplate, error) {
	if err := validateTemplate(req); err != nil {
		return model.EdgeTemplate{}, err
	}
	status := req.Status
	if status == "" {
		status = string(model.KnowledgeStatusDraft)
	}
	text := strings.TrimSpace(req.TemplateText)
	m := EdgeTemplateModel{
		TemplateID: TemplateID(text), TemplateText: text, RawExample: strings.TrimSpace(req.RawExample),
		Source: defaultString(req.Source, "manual"), EdgePred: req.EdgePred, Severity: req.Severity,
		Confidence: req.Confidence, EvidenceJSON: mustJSON(req.Evidence), AdviceCode: strings.TrimSpace(req.AdviceCode),
		Advice: strings.TrimSpace(req.Advice), Status: status,
	}
	if err := s.DB.WithContext(ctx).Create(&m).Error; err != nil {
		return model.EdgeTemplate{}, err
	}
	return templateFromModel(m), nil
}

func (s *EdgeKnowledgeStore) CreateTemplateFromCluster(ctx context.Context, req model.CreateTemplateFromClusterRequest) (model.EdgeTemplate, bool, error) {
	status := req.Status
	if status == "" {
		status = string(model.KnowledgeStatusDraft)
	}
	templateReq := model.UpsertEdgeTemplateRequest{
		TemplateText: strings.TrimSpace(req.TemplateText),
		RawExample:   strings.TrimSpace(req.SampleRawLog),
		Source:       defaultString(req.Source, "abnormal_cluster"),
		EdgePred:     req.EdgePred,
		Severity:     req.MaxSeverity,
		Confidence:   req.AvgConfidence,
		Evidence:     req.Evidence,
		AdviceCode:   strings.TrimSpace(req.AdviceCode),
		Advice:       strings.TrimSpace(req.Advice),
		Status:       status,
	}
	if err := validateTemplate(templateReq); err != nil {
		return model.EdgeTemplate{}, false, err
	}
	templateID := TemplateID(templateReq.TemplateText)
	var existing EdgeTemplateModel
	if err := s.DB.WithContext(ctx).First(&existing, "template_id = ? AND deleted_at IS NULL", templateID).Error; err == nil {
		return templateFromModel(existing), false, nil
	} else if err != gorm.ErrRecordNotFound {
		return model.EdgeTemplate{}, false, err
	}
	created, err := s.CreateTemplate(ctx, templateReq)
	return created, true, err
}

func (s *EdgeKnowledgeStore) PatchTemplate(ctx context.Context, id int64, req model.UpsertEdgeTemplateRequest) (model.EdgeTemplate, error) {
	if err := validateTemplate(req); err != nil {
		return model.EdgeTemplate{}, err
	}
	var m EdgeTemplateModel
	if err := s.DB.WithContext(ctx).First(&m, "id = ? AND deleted_at IS NULL", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return model.EdgeTemplate{}, ErrNotFound
		}
		return model.EdgeTemplate{}, err
	}
	text := strings.TrimSpace(req.TemplateText)
	m.TemplateID = TemplateID(text)
	m.TemplateText = text
	m.RawExample = strings.TrimSpace(req.RawExample)
	m.Source = defaultString(req.Source, "manual")
	m.EdgePred = req.EdgePred
	m.Severity = req.Severity
	m.Confidence = req.Confidence
	m.EvidenceJSON = mustJSON(req.Evidence)
	m.AdviceCode = strings.TrimSpace(req.AdviceCode)
	m.Advice = strings.TrimSpace(req.Advice)
	if req.Status != "" {
		m.Status = req.Status
	}
	if err := s.DB.WithContext(ctx).Save(&m).Error; err != nil {
		return model.EdgeTemplate{}, err
	}
	return templateFromModel(m), nil
}

func (s *EdgeKnowledgeStore) ListTemplates(ctx context.Context, q ListKnowledgeQuery) ([]model.EdgeTemplate, int, error) {
	page, ps := normalizePage(q.Page, q.PageSize)
	dbq := s.DB.WithContext(ctx).Model(&EdgeTemplateModel{}).Where("deleted_at IS NULL")
	if q.Status != "" {
		dbq = dbq.Where("status = ?", q.Status)
	}
	if q.EdgePred != "" {
		dbq = dbq.Where("edge_pred = ?", q.EdgePred)
	}
	if strings.TrimSpace(q.Q) != "" {
		like := "%" + strings.TrimSpace(q.Q) + "%"
		dbq = dbq.Where("template_text LIKE ? OR raw_example LIKE ? OR advice LIKE ?", like, like, like)
	}
	var total int64
	if err := dbq.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var ms []EdgeTemplateModel
	if err := dbq.Order("updated_at DESC").Limit(ps).Offset((page - 1) * ps).Find(&ms).Error; err != nil {
		return nil, 0, err
	}
	items := make([]model.EdgeTemplate, 0, len(ms))
	for _, m := range ms {
		items = append(items, templateFromModel(m))
	}
	return items, safeInt(total), nil
}

func (s *EdgeKnowledgeStore) DeleteTemplate(ctx context.Context, id int64) error {
	now := time.Now().UTC()
	res := s.DB.WithContext(ctx).Model(&EdgeTemplateModel{}).Where("id = ? AND deleted_at IS NULL", id).Updates(map[string]any{"status": string(model.KnowledgeStatusDeleted), "deleted_at": &now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *EdgeKnowledgeStore) CreateRuleSet(ctx context.Context, req model.CreateSetRequest) (model.EdgeRuleSet, error) {
	items, err := s.rulesByIDs(ctx, req.ItemIDs)
	if err != nil {
		return model.EdgeRuleSet{}, err
	}
	m := EdgeRuleSetModel{Name: strings.TrimSpace(req.Name), Description: strings.TrimSpace(req.Description), Status: string(model.SetStatusDraft), ItemIDsJSON: mustJSON(req.ItemIDs), ItemsJSON: mustJSON(items), ItemCount: len(items)}
	if err := s.DB.WithContext(ctx).Create(&m).Error; err != nil {
		return model.EdgeRuleSet{}, err
	}
	return ruleSetFromModel(m), nil
}

func (s *EdgeKnowledgeStore) ActivateRuleSet(ctx context.Context, id int64) (model.EdgeConfigRules, error) {
	var cfg model.EdgeConfigRules
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var m EdgeRuleSetModel
		if err := tx.First(&m, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return ErrNotFound
			}
			return err
		}
		version, err := nextVersion(tx, "edge_rule_sets")
		if err != nil {
			return err
		}
		m.Status = string(model.SetStatusActive)
		m.ConfigVersion = version
		m.Etag = makeEtag("rules", version, m.ItemsJSON)
		if err := tx.Model(&EdgeRuleSetModel{}).Where("status = ?", string(model.SetStatusActive)).Update("status", string(model.SetStatusArchived)).Error; err != nil {
			return err
		}
		if err := tx.Save(&m).Error; err != nil {
			return err
		}
		cfg.Version = m.ConfigVersion
		cfg.Etag = m.Etag
		_ = json.Unmarshal([]byte(m.ItemsJSON), &cfg.Items)
		return nil
	})
	return cfg, err
}

func (s *EdgeKnowledgeStore) CreateTemplateSet(ctx context.Context, req model.CreateSetRequest) (model.EdgeTemplateSet, error) {
	items, err := s.templatesByIDs(ctx, req.ItemIDs)
	if err != nil {
		return model.EdgeTemplateSet{}, err
	}
	m := EdgeTemplateSetModel{Name: strings.TrimSpace(req.Name), Description: strings.TrimSpace(req.Description), Status: string(model.SetStatusDraft), ItemIDsJSON: mustJSON(req.ItemIDs), ItemsJSON: mustJSON(items), ItemCount: len(items)}
	if err := s.DB.WithContext(ctx).Create(&m).Error; err != nil {
		return model.EdgeTemplateSet{}, err
	}
	return model.EdgeTemplateSet(ruleSetFromFields(m.ID, m.Name, m.Description, m.Status, m.ConfigVersion, m.Etag, m.ItemCount, m.CreatedAt, m.UpdatedAt)), nil
}

func (s *EdgeKnowledgeStore) ActivateTemplateSet(ctx context.Context, id int64) (model.EdgeConfigTemplates, error) {
	var cfg model.EdgeConfigTemplates
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var m EdgeTemplateSetModel
		if err := tx.First(&m, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return ErrNotFound
			}
			return err
		}
		version, err := nextVersion(tx, "edge_template_sets")
		if err != nil {
			return err
		}
		m.Status = string(model.SetStatusActive)
		m.ConfigVersion = version
		m.Etag = makeEtag("templates", version, m.ItemsJSON)
		if err := tx.Model(&EdgeTemplateSetModel{}).Where("status = ?", string(model.SetStatusActive)).Update("status", string(model.SetStatusArchived)).Error; err != nil {
			return err
		}
		if err := tx.Save(&m).Error; err != nil {
			return err
		}
		cfg.Version = m.ConfigVersion
		cfg.Etag = m.Etag
		_ = json.Unmarshal([]byte(m.ItemsJSON), &cfg.Items)
		return nil
	})
	return cfg, err
}

func (s *EdgeKnowledgeStore) ActiveRules(ctx context.Context) (model.EdgeConfigRules, error) {
	var m EdgeRuleSetModel
	if err := s.DB.WithContext(ctx).Order("config_version DESC").First(&m, "status = ?", string(model.SetStatusActive)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return model.EdgeConfigRules{Items: []model.EdgeRule{}}, nil
		}
		return model.EdgeConfigRules{}, err
	}
	var items []model.EdgeRule
	_ = json.Unmarshal([]byte(m.ItemsJSON), &items)
	return model.EdgeConfigRules{Version: m.ConfigVersion, Etag: m.Etag, Items: items}, nil
}

func (s *EdgeKnowledgeStore) ActiveTemplates(ctx context.Context) (model.EdgeConfigTemplates, error) {
	var m EdgeTemplateSetModel
	if err := s.DB.WithContext(ctx).Order("config_version DESC").First(&m, "status = ?", string(model.SetStatusActive)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return model.EdgeConfigTemplates{Items: []model.EdgeTemplate{}}, nil
		}
		return model.EdgeConfigTemplates{}, err
	}
	var items []model.EdgeTemplate
	_ = json.Unmarshal([]byte(m.ItemsJSON), &items)
	return model.EdgeConfigTemplates{Version: m.ConfigVersion, Etag: m.Etag, Items: items}, nil
}

func (s *EdgeKnowledgeStore) rulesByIDs(ctx context.Context, ids []int64) ([]model.EdgeRule, error) {
	var ms []EdgeRuleModel
	q := s.DB.WithContext(ctx).Where("deleted_at IS NULL AND status <> ?", string(model.KnowledgeStatusDeleted))
	if len(ids) > 0 {
		q = q.Where("id IN ?", ids)
	}
	if err := q.Order("priority ASC").Find(&ms).Error; err != nil {
		return nil, err
	}
	items := make([]model.EdgeRule, 0, len(ms))
	for _, m := range ms {
		if m.Enabled {
			items = append(items, ruleFromModel(m))
		}
	}
	return items, nil
}

func (s *EdgeKnowledgeStore) templatesByIDs(ctx context.Context, ids []int64) ([]model.EdgeTemplate, error) {
	var ms []EdgeTemplateModel
	q := s.DB.WithContext(ctx).Where("deleted_at IS NULL AND status <> ?", string(model.KnowledgeStatusDeleted))
	if len(ids) > 0 {
		q = q.Where("id IN ?", ids)
	}
	if err := q.Order("updated_at DESC").Find(&ms).Error; err != nil {
		return nil, err
	}
	items := make([]model.EdgeTemplate, 0, len(ms))
	for _, m := range ms {
		items = append(items, templateFromModel(m))
	}
	return items, nil
}

func validateRule(req model.UpsertEdgeRuleRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return ErrBadRequest
	}
	if !validPred(req.EdgePred) || req.Severity < 0 || req.Severity > 100 || req.Confidence < 0 || req.Confidence > 1 {
		return ErrBadRequest
	}
	mt := strings.TrimSpace(req.MatchType)
	if mt != "contains" && mt != "regex" && mt != "all_keywords" && mt != "any_keywords" {
		return ErrBadRequest
	}
	if mt == "regex" {
		if _, err := regexp.Compile(req.Pattern); err != nil {
			return fmt.Errorf("%w: invalid regex", ErrBadRequest)
		}
	}
	if (mt == "contains" || mt == "regex") && strings.TrimSpace(req.Pattern) == "" {
		return ErrBadRequest
	}
	if (mt == "all_keywords" || mt == "any_keywords") && len(req.Keywords) == 0 {
		return ErrBadRequest
	}
	return nil
}

func validateTemplate(req model.UpsertEdgeTemplateRequest) error {
	if strings.TrimSpace(req.TemplateText) == "" {
		return ErrBadRequest
	}
	if !validPred(req.EdgePred) || req.Severity < 0 || req.Severity > 100 || req.Confidence < 0 || req.Confidence > 1 {
		return ErrBadRequest
	}
	return nil
}

func validPred(v string) bool {
	switch v {
	case "NORMAL", "ABNORMAL", "APP_ERROR", "NET_ERROR", "DISK_ERROR", "INFRA_ERROR", "SECURITY", "UNKNOWN":
		return true
	default:
		return false
	}
}

func ruleFromModel(m EdgeRuleModel) model.EdgeRule {
	var keywords, evidence []string
	_ = json.Unmarshal([]byte(m.KeywordsJSON), &keywords)
	_ = json.Unmarshal([]byte(m.EvidenceJSON), &evidence)
	return model.EdgeRule{ID: m.ID, Name: m.Name, Description: m.Description, MatchType: m.MatchType, Pattern: m.Pattern, Keywords: keywords, TargetField: m.TargetField, EdgePred: m.EdgePred, Severity: m.Severity, Confidence: m.Confidence, Evidence: evidence, AdviceCode: m.AdviceCode, Advice: m.Advice, Priority: m.Priority, Enabled: m.Enabled, Status: model.KnowledgeStatus(m.Status), CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}
}

func templateFromModel(m EdgeTemplateModel) model.EdgeTemplate {
	var evidence []string
	_ = json.Unmarshal([]byte(m.EvidenceJSON), &evidence)
	return model.EdgeTemplate{ID: m.ID, TemplateID: m.TemplateID, TemplateText: m.TemplateText, RawExample: m.RawExample, Source: m.Source, EdgePred: m.EdgePred, Severity: m.Severity, Confidence: m.Confidence, Evidence: evidence, AdviceCode: m.AdviceCode, Advice: m.Advice, Status: model.KnowledgeStatus(m.Status), CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}
}

func ruleSetFromModel(m EdgeRuleSetModel) model.EdgeRuleSet {
	return ruleSetFromFields(m.ID, m.Name, m.Description, m.Status, m.ConfigVersion, m.Etag, m.ItemCount, m.CreatedAt, m.UpdatedAt)
}

func ruleSetFromFields(id int64, name, desc, status string, version int64, etag string, count int, created, updated time.Time) model.EdgeRuleSet {
	return model.EdgeRuleSet{ID: id, Name: name, Description: desc, Status: model.SetStatus(status), ConfigVersion: version, Etag: etag, ItemCount: count, CreatedAt: created, UpdatedAt: updated}
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func defaultString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return strings.TrimSpace(v)
}

func normalizePage(page, ps int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if ps <= 0 {
		ps = 20
	}
	if ps > 200 {
		ps = 200
	}
	return page, ps
}

func safeInt(v int64) int {
	if v > math.MaxInt {
		return math.MaxInt
	}
	return int(v)
}

func TemplateID(text string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(text)))
	return hex.EncodeToString(sum[:])[:16]
}

func makeEtag(prefix string, version int64, body string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s", prefix, version, body)))
	return fmt.Sprintf("W/\"%s-%d-%s\"", prefix, version, hex.EncodeToString(sum[:])[:8])
}

func nextVersion(tx *gorm.DB, table string) (int64, error) {
	var maxVersion int64
	if err := tx.Table(table).Select("COALESCE(MAX(config_version), 0)").Scan(&maxVersion).Error; err != nil {
		return 0, err
	}
	return maxVersion + 1, nil
}
