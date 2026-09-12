package store

import "time"

type ClassifierConfigModel struct {
	ID            int64      `gorm:"primaryKey;autoIncrement"`
	Name          string     `gorm:"size:255;not null;uniqueIndex:uk_classifier_configs_name"`
	Endpoint      string     `gorm:"type:text;not null"`
	Model         string     `gorm:"size:255;not null"`
	TimeoutMS     int        `gorm:"not null"`
	Threshold     float64    `gorm:"not null"`
	Status        string     `gorm:"size:32;not null;index"`
	IsActive      bool       `gorm:"not null;index"`
	ConfigVersion int64      `gorm:"not null"`
	Etag          string     `gorm:"size:255;not null"`
	CreatedAt     time.Time  `gorm:"autoCreateTime"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime"`
	DeletedAt     *time.Time `gorm:"index"`
}

func (ClassifierConfigModel) TableName() string { return "classifier_configs" }
