package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/pangjian-pj/cotriage-dev/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrConflict   = errors.New("conflict")
	ErrBadRequest = errors.New("bad request")
)

type EdgeProvidersStore struct {
	DB *gorm.DB
}

func NewEdgeProvidersStore(db *gorm.DB) *EdgeProvidersStore {
	return &EdgeProvidersStore{DB: db}
}

func (s *EdgeProvidersStore) Create(ctx context.Context, req model.CreateEdgeProviderRequest) (model.EdgeProvider, error) {
	now := time.Now().UTC()
	m := EdgeProviderModel{
		Name:          strings.TrimSpace(req.Name),
		Provider:      string(req.Provider),
		Endpoint:      strings.TrimSpace(req.Endpoint),
		Model:         strings.TrimSpace(req.Model),
		TimeoutMS:     req.TimeoutMS,
		MaxTokens:     req.MaxTokens,
		Temperature:   req.Temperature,
		TopP:          req.TopP,
		Seed:          req.Seed,
		HealthPath:    req.HealthPath,
		ResponsePath:  strings.TrimSpace(req.ResponsePath),
		Status:        string(model.EdgeProviderStatusDraft),
		IsActive:      false,
		ConfigVersion: 0,
		Etag:          "",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.DB.WithContext(ctx).Create(&m).Error; err != nil {
		return model.EdgeProvider{}, err
	}
	return s.Get(ctx, m.ID)
}

func (s *EdgeProvidersStore) Get(ctx context.Context, id int64) (model.EdgeProvider, error) {
	var m EdgeProviderModel
	err := s.DB.WithContext(ctx).First(&m, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.EdgeProvider{}, ErrNotFound
	}
	if err != nil {
		return model.EdgeProvider{}, err
	}
	return edgeProviderFromModel(m), nil
}

type ListEdgeProvidersQuery struct {
	Status   *model.EdgeProviderStatus
	Active   *bool
	Q        string
	Page     int
	PageSize int
}

func (s *EdgeProvidersStore) List(ctx context.Context, q ListEdgeProvidersQuery) ([]model.EdgeProvider, int, error) {
	page := q.Page
	if page <= 0 {
		page = 1
	}
	ps := q.PageSize
	if ps <= 0 {
		ps = 20
	}
	if ps > 200 {
		ps = 200
	}

	qdb := s.DB.WithContext(ctx).Model(&EdgeProviderModel{}).Where("deleted_at IS NULL")
	if q.Status != nil && strings.TrimSpace(string(*q.Status)) != "" {
		qdb = qdb.Where("status = ?", string(*q.Status))
	}
	if q.Active != nil {
		qdb = qdb.Where("is_active = ?", *q.Active)
	}
	if strings.TrimSpace(q.Q) != "" {
		qdb = qdb.Where("name LIKE ?", "%"+strings.TrimSpace(q.Q)+"%")
	}

	var total int64
	if err := qdb.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var ms []EdgeProviderModel
	if err := qdb.Order("updated_at DESC").Order("id DESC").Limit(ps).Offset((page - 1) * ps).Find(&ms).Error; err != nil {
		return nil, 0, err
	}
	items := make([]model.EdgeProvider, 0, len(ms))
	for _, m := range ms {
		items = append(items, edgeProviderFromModel(m))
	}
	// Avoid overflow for api response.
	if total > math.MaxInt {
		total = math.MaxInt
	}
	return items, int(total), nil
}

func (s *EdgeProvidersStore) Patch(ctx context.Context, id int64, req model.PatchEdgeProviderRequest) (model.EdgeProvider, error) {
	cur, err := s.Get(ctx, id)
	if err != nil {
		return model.EdgeProvider{}, err
	}

	// Apply patch in memory then update.
	if req.Name != nil {
		cur.Name = strings.TrimSpace(*req.Name)
	}
	if req.Provider != nil {
		cur.Provider = *req.Provider
	}
	if req.Endpoint != nil {
		cur.Endpoint = strings.TrimSpace(*req.Endpoint)
	}
	if req.Model != nil {
		cur.Model = strings.TrimSpace(*req.Model)
	}
	if req.TimeoutMS != nil {
		cur.TimeoutMS = *req.TimeoutMS
	}
	if req.MaxTokens != nil {
		cur.MaxTokens = *req.MaxTokens
	}
	if req.Temperature != nil {
		cur.Temperature = *req.Temperature
	}
	if req.TopP != nil {
		cur.TopP = *req.TopP
	}
	if req.Seed != nil {
		cur.Seed = *req.Seed
	}
	if req.HealthPath != nil {
		cur.HealthPath = *req.HealthPath
	}
	if req.ResponsePath != nil {
		cur.ResponsePath = strings.TrimSpace(*req.ResponsePath)
	}
	if req.Status != nil {
		cur.Status = *req.Status
	}
	if req.IsActive != nil {
		cur.IsActive = *req.IsActive
	}

	// Enforce invariants.
	if cur.IsActive {
		if cur.Status != model.EdgeProviderStatusPublished {
			return model.EdgeProvider{}, ErrBadRequest
		}
	}
	if cur.Status == model.EdgeProviderStatusDisabled || cur.Status == model.EdgeProviderStatusDeleted || cur.Status == model.EdgeProviderStatusDraft {
		cur.IsActive = false
	}

	now := time.Now().UTC()
	updates := map[string]any{
		"name":          cur.Name,
		"provider":      string(cur.Provider),
		"endpoint":      cur.Endpoint,
		"model":         cur.Model,
		"timeout_ms":    cur.TimeoutMS,
		"max_tokens":    cur.MaxTokens,
		"temperature":   cur.Temperature,
		"top_p":         cur.TopP,
		"seed":          cur.Seed,
		"health_path":   cur.HealthPath,
		"response_path": cur.ResponsePath,
		"status":        string(cur.Status),
		"is_active":     cur.IsActive,
		"updated_at":    now,
	}
	res := s.DB.WithContext(ctx).Model(&EdgeProviderModel{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return model.EdgeProvider{}, res.Error
	}
	if res.RowsAffected == 0 {
		return model.EdgeProvider{}, ErrNotFound
	}
	return s.Get(ctx, id)
}

func (s *EdgeProvidersStore) SoftDelete(ctx context.Context, id int64) error {
	p, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if p.IsActive {
		return ErrConflict
	}
	now := time.Now().UTC()
	res := s.DB.WithContext(ctx).Model(&EdgeProviderModel{}).Where("id = ?", id).Updates(map[string]any{
		"status":     string(model.EdgeProviderStatusDeleted),
		"deleted_at": now,
		"updated_at": now,
		"is_active":  false,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *EdgeProvidersStore) Activate(ctx context.Context, id int64) (model.EdgeConfigResponse, error) {
	// Must exist.
	p, err := s.Get(ctx, id)
	if err != nil {
		return model.EdgeConfigResponse{}, err
	}
	if p.Status == model.EdgeProviderStatusDisabled || p.Status == model.EdgeProviderStatusDeleted {
		return model.EdgeConfigResponse{}, ErrBadRequest
	}

	var out model.EdgeConfigResponse
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock table rows to avoid concurrent activate causing two actives/config_version race.
		var next int64
		if err := tx.Model(&EdgeProviderModel{}).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("COALESCE(MAX(config_version), 0)+1").
			Scan(&next).Error; err != nil {
			return err
		}

		// Deactivate any current active.
		if err := tx.Model(&EdgeProviderModel{}).Where("is_active = ?", true).Update("is_active", false).Error; err != nil {
			return err
		}

		now := time.Now().UTC()
		tag := weakEtag(next, p)
		res := tx.Model(&EdgeProviderModel{}).
			Where("id = ? AND deleted_at IS NULL", id).
			Updates(map[string]any{
				"is_active":       true,
				"status":          string(model.EdgeProviderStatusPublished),
				"config_version":  next,
				"etag":            tag,
				"last_publish_at": now,
				"updated_at":      now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}

		active, err := s.Get(ctx, id)
		if err != nil {
			return err
		}
		out = model.EdgeConfigResponse{ConfigVersion: next, Etag: tag, ActiveProvider: &active}
		return nil
	})
	if err != nil {
		return model.EdgeConfigResponse{}, err
	}
	return out, nil
}

func (s *EdgeProvidersStore) GetActiveConfig(ctx context.Context) (model.EdgeConfigResponse, error) {
	// Find active published.
	var m EdgeProviderModel
	err := s.DB.WithContext(ctx).
		Where("is_active = ? AND status = ? AND deleted_at IS NULL", true, string(model.EdgeProviderStatusPublished)).
		Limit(1).
		Find(&m).Error
	if err != nil {
		return model.EdgeConfigResponse{}, err
	}
	if m.ID == 0 {
		return model.EdgeConfigResponse{ConfigVersion: 0, Etag: "", ActiveProvider: nil}, nil
	}
	p := edgeProviderFromModel(m)
	return model.EdgeConfigResponse{ConfigVersion: p.ConfigVersion, Etag: p.Etag, ActiveProvider: &p}, nil
}

func edgeProviderFromModel(m EdgeProviderModel) model.EdgeProvider {
	p := model.EdgeProvider{
		ID:            m.ID,
		Name:          m.Name,
		Provider:      model.EdgeProviderType(m.Provider),
		Endpoint:      m.Endpoint,
		Model:         m.Model,
		TimeoutMS:     m.TimeoutMS,
		MaxTokens:     m.MaxTokens,
		Temperature:   m.Temperature,
		TopP:          m.TopP,
		Seed:          m.Seed,
		HealthPath:    m.HealthPath,
		ResponsePath:  m.ResponsePath,
		Status:        model.EdgeProviderStatus(m.Status),
		IsActive:      m.IsActive,
		ConfigVersion: m.ConfigVersion,
		Etag:          m.Etag,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
		CreatedBy:     m.CreatedBy,
		UpdatedBy:     m.UpdatedBy,
		DeletedAt:     m.DeletedAt,
		LastPublishAt: m.LastPublishAt,
	}
	return p
}

func weakEtag(version int64, p model.EdgeProvider) string {
	h := sha256.New()
	_, _ = h.Write([]byte(fmt.Sprintf("%d|%s|%s|%s|%s|%d|%d|%f|%f|%s|%s", version, p.Name, p.Provider, p.Endpoint, p.Model, p.TimeoutMS, p.MaxTokens, p.Temperature, p.TopP, derefStr(p.HealthPath), p.ResponsePath)))
	if p.Seed != nil {
		_, _ = h.Write([]byte(fmt.Sprintf("|seed=%d", *p.Seed)))
	}
	sum := hex.EncodeToString(h.Sum(nil))
	// Example: W/"104-9ab122"
	return fmt.Sprintf("W/\"%d-%s\"", version, sum[:6])
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
