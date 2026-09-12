package store

import "time"

// EdgeProviderModel is the GORM model mapped to `edge_providers` table.
// We keep it separate from the API model (`internal/model.EdgeProvider`) to avoid coupling JSON and DB tags.
//
// Notes:
// - Soft delete: we use DeletedAt pointer but NOT gorm.DeletedAt, because status has its own lifecycle.
// - Indices follow the design doc (name unique; status/active/deleted queries).
type EdgeProviderModel struct {
	ID            int64      `gorm:"primaryKey;autoIncrement"`
	Name          string     `gorm:"size:255;not null;uniqueIndex:uk_edge_providers_name"`
	Provider      string     `gorm:"size:32;not null"`
	Endpoint      string     `gorm:"type:text;not null"`
	Model         string     `gorm:"size:255;not null"`
	TimeoutMS     int        `gorm:"not null"`
	MaxTokens     int        `gorm:"not null"`
	Temperature   float64    `gorm:"not null"`
	TopP          float64    `gorm:"not null"`
	Seed          *int       `gorm:""`
	HealthPath    *string    `gorm:"size:255"`
	ResponsePath  string     `gorm:"size:255;not null"`
	Status        string     `gorm:"size:32;not null;index:idx_edge_providers_status"`
	IsActive      bool       `gorm:"not null;index:idx_edge_providers_active"`
	ConfigVersion int64      `gorm:"not null"`
	Etag          string     `gorm:"size:255;not null"`
	CreatedAt     time.Time  `gorm:"autoCreateTime"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime"`
	CreatedBy     *string    `gorm:"size:255"`
	UpdatedBy     *string    `gorm:"size:255"`
	DeletedAt     *time.Time `gorm:"index:idx_edge_providers_deleted"`
	LastPublishAt *time.Time `gorm:""`
}

func (EdgeProviderModel) TableName() string { return "edge_providers" }
