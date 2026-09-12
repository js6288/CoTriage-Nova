package store

import (
	"context"
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/pangjian-pj/cotriage-dev/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CloudProviderModel struct {
	ID            int64   `gorm:"primaryKey;autoIncrement"`
	Name          string  `gorm:"size:255;not null;uniqueIndex:uk_cloud_providers_name"`
	Provider      string  `gorm:"size:32;not null"`
	Endpoint      string  `gorm:"type:text;not null"`
	Model         string  `gorm:"size:255;not null"`
	APIKeyEnv     string  `gorm:"size:255;not null"`
	APIKey        string  `gorm:"type:text;not null"`
	TimeoutMS     int     `gorm:"not null"`
	MaxTokens     int     `gorm:"not null"`
	Temperature   float64 `gorm:"not null"`
	TopP          float64 `gorm:"not null"`
	Seed          *int
	HealthPath    *string `gorm:"size:255"`
	ResponsePath  string  `gorm:"size:255;not null"`
	Status        string  `gorm:"size:32;not null;index"`
	IsActive      bool    `gorm:"not null;index"`
	ConfigVersion int64   `gorm:"not null"`
	Etag          string  `gorm:"size:255;not null"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	CreatedBy     *string    `gorm:"size:255"`
	UpdatedBy     *string    `gorm:"size:255"`
	DeletedAt     *time.Time `gorm:"index"`
	LastPublishAt *time.Time
}

func (CloudProviderModel) TableName() string { return "cloud_providers" }

type CloudProvidersStore struct{ DB *gorm.DB }

func NewCloudProvidersStore(db *gorm.DB, masterKey ...string) *CloudProvidersStore {
	return &CloudProvidersStore{DB: db}
}

var environmentVariableName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func validateCloudProvider(p model.CloudProvider) error {
	switch p.Provider {
	case "openai_compat", "deepseek", "qwen", "custom_http":
	default:
		return ErrBadRequest
	}
	if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Endpoint) == "" || strings.TrimSpace(p.Model) == "" || !environmentVariableName.MatchString(strings.TrimSpace(p.APIKeyEnv)) || p.TimeoutMS <= 0 || p.MaxTokens <= 0 {
		return ErrBadRequest
	}
	if p.Temperature < 0 || p.Temperature > 2 || p.TopP <= 0 || p.TopP > 1 {
		return ErrBadRequest
	}
	return nil
}

func (s *CloudProvidersStore) Create(ctx context.Context, r model.CreateCloudProviderRequest) (model.CloudProvider, error) {
	p := model.CloudProvider{Name: strings.TrimSpace(r.Name), Provider: strings.TrimSpace(r.Provider), Endpoint: strings.TrimSpace(r.Endpoint), Model: strings.TrimSpace(r.Model), APIKeyEnv: strings.TrimSpace(r.APIKeyEnv), TimeoutMS: r.TimeoutMS, MaxTokens: r.MaxTokens, Temperature: r.Temperature, TopP: r.TopP, Seed: r.Seed, HealthPath: r.HealthPath, ResponsePath: strings.TrimSpace(r.ResponsePath), Status: "draft"}
	if p.TopP == 0 {
		p.TopP = .9
	}
	if p.ResponsePath == "" {
		p.ResponsePath = "choices[0].message.content"
	}
	if err := validateCloudProvider(p); err != nil {
		return p, err
	}
	now := time.Now().UTC()
	m := cloudModel(p)
	m.APIKey = strings.TrimSpace(r.APIKey)
	m.CreatedAt = now
	m.UpdatedAt = now
	if err := s.DB.WithContext(ctx).Create(&m).Error; err != nil {
		return p, err
	}
	return s.Get(ctx, m.ID)
}

func (s *CloudProvidersStore) Get(ctx context.Context, id int64) (model.CloudProvider, error) {
	var m CloudProviderModel
	err := s.DB.WithContext(ctx).Where("id=? AND deleted_at IS NULL", id).First(&m).Error
	if err == gorm.ErrRecordNotFound {
		return model.CloudProvider{}, ErrNotFound
	}
	return cloudFromModel(m), err
}
func (s *CloudProvidersStore) List(ctx context.Context) ([]model.CloudProvider, error) {
	var ms []CloudProviderModel
	if err := s.DB.WithContext(ctx).Where("deleted_at IS NULL").Order("updated_at desc").Find(&ms).Error; err != nil {
		return nil, err
	}
	out := make([]model.CloudProvider, 0, len(ms))
	for _, m := range ms {
		out = append(out, cloudFromModel(m))
	}
	return out, nil
}

func (s *CloudProvidersStore) Patch(ctx context.Context, id int64, r model.PatchCloudProviderRequest) (model.CloudProvider, error) {
	p, err := s.Get(ctx, id)
	if err != nil {
		return p, err
	}
	if r.Name != nil {
		p.Name = strings.TrimSpace(*r.Name)
	}
	if r.Provider != nil {
		p.Provider = strings.TrimSpace(*r.Provider)
	}
	if r.Endpoint != nil {
		p.Endpoint = strings.TrimSpace(*r.Endpoint)
	}
	if r.Model != nil {
		p.Model = strings.TrimSpace(*r.Model)
	}
	if r.APIKeyEnv != nil {
		p.APIKeyEnv = strings.TrimSpace(*r.APIKeyEnv)
	}
	var newAPIKey *string
	if r.APIKey != nil && strings.TrimSpace(*r.APIKey) != "" {
		value := strings.TrimSpace(*r.APIKey)
		newAPIKey = &value
	}
	if r.TimeoutMS != nil {
		p.TimeoutMS = *r.TimeoutMS
	}
	if r.MaxTokens != nil {
		p.MaxTokens = *r.MaxTokens
	}
	if r.Temperature != nil {
		p.Temperature = *r.Temperature
	}
	if r.TopP != nil {
		p.TopP = *r.TopP
	}
	if r.Seed != nil {
		p.Seed = *r.Seed
	}
	if r.HealthPath != nil {
		p.HealthPath = *r.HealthPath
	}
	if r.ResponsePath != nil {
		p.ResponsePath = strings.TrimSpace(*r.ResponsePath)
	}
	if r.Status != nil {
		p.Status = *r.Status
	}
	if p.IsActive {
		return p, ErrConflict
	}
	if p.Status != "draft" && p.Status != "disabled" && p.Status != "published" {
		return p, ErrBadRequest
	}
	if err := validateCloudProvider(p); err != nil {
		return p, err
	}
	m := cloudModel(p)
	updates := map[string]any{"name": m.Name, "provider": m.Provider, "endpoint": m.Endpoint, "model": m.Model, "api_key_env": m.APIKeyEnv, "timeout_ms": m.TimeoutMS, "max_tokens": m.MaxTokens, "temperature": m.Temperature, "top_p": m.TopP, "seed": m.Seed, "health_path": m.HealthPath, "response_path": m.ResponsePath, "status": m.Status, "updated_at": time.Now().UTC()}
	if newAPIKey != nil {
		updates["api_key"] = *newAPIKey
	}
	err = s.DB.WithContext(ctx).Model(&CloudProviderModel{}).Where("id=?", id).Updates(updates).Error
	if err != nil {
		return p, err
	}
	return s.Get(ctx, id)
}
func (s *CloudProvidersStore) Delete(ctx context.Context, id int64) error {
	p, e := s.Get(ctx, id)
	if e != nil {
		return e
	}
	if p.IsActive {
		return ErrConflict
	}
	now := time.Now().UTC()
	return s.DB.WithContext(ctx).Model(&CloudProviderModel{}).Where("id=?", id).Updates(map[string]any{"status": "deleted", "deleted_at": now, "updated_at": now}).Error
}
func (s *CloudProvidersStore) Activate(ctx context.Context, id int64) (model.CloudProvider, error) {
	p, e := s.Get(ctx, id)
	if e != nil {
		return p, e
	}
	if e = validateCloudProvider(p); e != nil {
		return p, e
	}
	e = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var v int64
		if e := tx.Model(&CloudProviderModel{}).Clauses(clause.Locking{Strength: "UPDATE"}).Select("COALESCE(MAX(config_version),0)+1").Scan(&v).Error; e != nil {
			return e
		}
		if e := tx.Model(&CloudProviderModel{}).Where("is_active=?", true).Update("is_active", false).Error; e != nil {
			return e
		}
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%d", p.Endpoint, p.Model, p.APIKeyEnv, v)))
		tag := fmt.Sprintf(`W/"cloud-config-%d-%x"`, v, sum[:4])
		now := time.Now().UTC()
		return tx.Model(&CloudProviderModel{}).Where("id=?", id).Updates(map[string]any{"is_active": true, "status": "published", "config_version": v, "etag": tag, "last_publish_at": now, "updated_at": now}).Error
	})
	if e != nil {
		return p, e
	}
	return s.Get(ctx, id)
}
func (s *CloudProvidersStore) Active(ctx context.Context) (*model.CloudProvider, error) {
	var m CloudProviderModel
	if e := s.DB.WithContext(ctx).Where("is_active=? AND status=? AND deleted_at IS NULL", true, "published").First(&m).Error; e == gorm.ErrRecordNotFound {
		return nil, nil
	} else if e != nil {
		return nil, e
	}
	p := cloudFromModel(m)
	return &p, nil
}

func cloudModel(p model.CloudProvider) CloudProviderModel {
	return CloudProviderModel{ID: p.ID, Name: p.Name, Provider: p.Provider, Endpoint: p.Endpoint, Model: p.Model, APIKeyEnv: p.APIKeyEnv, APIKey: p.APIKey, TimeoutMS: p.TimeoutMS, MaxTokens: p.MaxTokens, Temperature: p.Temperature, TopP: p.TopP, Seed: p.Seed, HealthPath: p.HealthPath, ResponsePath: p.ResponsePath, Status: p.Status, IsActive: p.IsActive, ConfigVersion: p.ConfigVersion, Etag: p.Etag, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt, DeletedAt: p.DeletedAt, LastPublishAt: p.LastPublishAt}
}
func cloudFromModel(m CloudProviderModel) model.CloudProvider {
	return model.CloudProvider{ID: m.ID, Name: m.Name, Provider: m.Provider, Endpoint: m.Endpoint, Model: m.Model, APIKeyEnv: m.APIKeyEnv, APIKey: m.APIKey, TimeoutMS: m.TimeoutMS, MaxTokens: m.MaxTokens, Temperature: m.Temperature, TopP: m.TopP, Seed: m.Seed, HealthPath: m.HealthPath, ResponsePath: m.ResponsePath, Status: m.Status, IsActive: m.IsActive, ConfigVersion: m.ConfigVersion, Etag: m.Etag, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt, DeletedAt: m.DeletedAt, LastPublishAt: m.LastPublishAt}
}
