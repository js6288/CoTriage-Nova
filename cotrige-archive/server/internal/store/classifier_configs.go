package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/pangjian-pj/cotriage-dev/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ClassifierConfigsStore struct {
	DB *gorm.DB
}

func NewClassifierConfigsStore(db *gorm.DB) *ClassifierConfigsStore {
	return &ClassifierConfigsStore{DB: db}
}

func (s *ClassifierConfigsStore) Create(ctx context.Context, req model.UpsertClassifierConfigRequest) (model.ClassifierConfig, error) {
	if err := validateClassifier(req); err != nil {
		return model.ClassifierConfig{}, err
	}
	status := req.Status
	if status == "" {
		status = string(model.EdgeProviderStatusDraft)
	}
	m := ClassifierConfigModel{
		Name:      strings.TrimSpace(req.Name),
		Endpoint:  strings.TrimSpace(req.Endpoint),
		Model:     strings.TrimSpace(req.Model),
		TimeoutMS: req.TimeoutMS,
		Threshold: req.Threshold,
		Status:    status,
		IsActive:  false,
	}
	if err := s.DB.WithContext(ctx).Create(&m).Error; err != nil {
		return model.ClassifierConfig{}, err
	}
	return classifierFromModel(m), nil
}

func (s *ClassifierConfigsStore) Patch(ctx context.Context, id int64, req model.UpsertClassifierConfigRequest) (model.ClassifierConfig, error) {
	if err := validateClassifier(req); err != nil {
		return model.ClassifierConfig{}, err
	}
	var m ClassifierConfigModel
	if err := s.DB.WithContext(ctx).First(&m, "id = ? AND deleted_at IS NULL", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return model.ClassifierConfig{}, ErrNotFound
		}
		return model.ClassifierConfig{}, err
	}
	m.Name = strings.TrimSpace(req.Name)
	m.Endpoint = strings.TrimSpace(req.Endpoint)
	m.Model = strings.TrimSpace(req.Model)
	m.TimeoutMS = req.TimeoutMS
	m.Threshold = req.Threshold
	if req.Status != "" {
		m.Status = req.Status
	}
	if m.Status == string(model.EdgeProviderStatusDraft) || m.Status == string(model.EdgeProviderStatusDisabled) || m.Status == string(model.EdgeProviderStatusDeleted) {
		m.IsActive = false
	}
	if err := s.DB.WithContext(ctx).Save(&m).Error; err != nil {
		return model.ClassifierConfig{}, err
	}
	return classifierFromModel(m), nil
}

func (s *ClassifierConfigsStore) List(ctx context.Context, q ListKnowledgeQuery) ([]model.ClassifierConfig, int, error) {
	page, ps := normalizePage(q.Page, q.PageSize)
	dbq := s.DB.WithContext(ctx).Model(&ClassifierConfigModel{}).Where("deleted_at IS NULL")
	if q.Status != "" {
		dbq = dbq.Where("status = ?", q.Status)
	}
	if strings.TrimSpace(q.Q) != "" {
		like := "%" + strings.TrimSpace(q.Q) + "%"
		dbq = dbq.Where("name LIKE ? OR model LIKE ? OR endpoint LIKE ?", like, like, like)
	}
	var total int64
	if err := dbq.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var ms []ClassifierConfigModel
	if err := dbq.Order("updated_at DESC").Limit(ps).Offset((page - 1) * ps).Find(&ms).Error; err != nil {
		return nil, 0, err
	}
	items := make([]model.ClassifierConfig, 0, len(ms))
	for _, m := range ms {
		items = append(items, classifierFromModel(m))
	}
	if total > math.MaxInt {
		total = math.MaxInt
	}
	return items, int(total), nil
}

func (s *ClassifierConfigsStore) Delete(ctx context.Context, id int64) error {
	var m ClassifierConfigModel
	if err := s.DB.WithContext(ctx).First(&m, "id = ? AND deleted_at IS NULL", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrNotFound
		}
		return err
	}
	if m.IsActive {
		return ErrConflict
	}
	now := time.Now().UTC()
	res := s.DB.WithContext(ctx).Model(&ClassifierConfigModel{}).Where("id = ?", id).Updates(map[string]any{
		"status":     string(model.EdgeProviderStatusDeleted),
		"deleted_at": &now,
		"updated_at": now,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *ClassifierConfigsStore) Activate(ctx context.Context, id int64) (model.ClassifierConfig, error) {
	var out model.ClassifierConfig
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cur ClassifierConfigModel
		if err := tx.First(&cur, "id = ? AND deleted_at IS NULL", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return ErrNotFound
			}
			return err
		}
		if cur.Status == string(model.EdgeProviderStatusDisabled) || cur.Status == string(model.EdgeProviderStatusDeleted) {
			return ErrBadRequest
		}
		var next int64
		if err := tx.Model(&ClassifierConfigModel{}).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("COALESCE(MAX(config_version), 0)+1").
			Scan(&next).Error; err != nil {
			return err
		}
		if err := tx.Model(&ClassifierConfigModel{}).Where("is_active = ?", true).Update("is_active", false).Error; err != nil {
			return err
		}
		tag := classifierEtag(next, cur)
		now := time.Now().UTC()
		if err := tx.Model(&ClassifierConfigModel{}).Where("id = ?", id).Updates(map[string]any{
			"status":         string(model.EdgeProviderStatusPublished),
			"is_active":      true,
			"config_version": next,
			"etag":           tag,
			"updated_at":     now,
		}).Error; err != nil {
			return err
		}
		if err := tx.First(&cur, "id = ?", id).Error; err != nil {
			return err
		}
		out = classifierFromModel(cur)
		return nil
	})
	return out, err
}

func (s *ClassifierConfigsStore) Active(ctx context.Context) (*model.ClassifierConfig, error) {
	var m ClassifierConfigModel
	if err := s.DB.WithContext(ctx).First(&m, "is_active = ? AND status = ? AND deleted_at IS NULL", true, string(model.EdgeProviderStatusPublished)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	item := classifierFromModel(m)
	return &item, nil
}

func validateClassifier(req model.UpsertClassifierConfigRequest) error {
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Endpoint) == "" || strings.TrimSpace(req.Model) == "" {
		return ErrBadRequest
	}
	if req.TimeoutMS <= 0 || req.Threshold < 0 || req.Threshold > 1 {
		return ErrBadRequest
	}
	if req.Status != "" {
		switch req.Status {
		case string(model.EdgeProviderStatusDraft), string(model.EdgeProviderStatusPublished), string(model.EdgeProviderStatusDisabled), string(model.EdgeProviderStatusDeleted):
		default:
			return ErrBadRequest
		}
	}
	return nil
}

func classifierFromModel(m ClassifierConfigModel) model.ClassifierConfig {
	return model.ClassifierConfig{
		ID:            m.ID,
		Name:          m.Name,
		Endpoint:      m.Endpoint,
		Model:         m.Model,
		TimeoutMS:     m.TimeoutMS,
		Threshold:     m.Threshold,
		Status:        m.Status,
		IsActive:      m.IsActive,
		ConfigVersion: m.ConfigVersion,
		Etag:          m.Etag,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
}

func classifierEtag(version int64, m ClassifierConfigModel) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d|%s|%s|%s|%d|%f", version, m.Name, m.Endpoint, m.Model, m.TimeoutMS, m.Threshold)))
	return fmt.Sprintf("W/\"classifier-%d-%s\"", version, hex.EncodeToString(sum[:])[:8])
}
