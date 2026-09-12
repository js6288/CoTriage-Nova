package store

import "time"

type EdgeRuleModel struct {
	ID           int64      `gorm:"primaryKey;autoIncrement"`
	Name         string     `gorm:"size:255;not null;index"`
	Description  string     `gorm:"type:text"`
	MatchType    string     `gorm:"size:32;not null"`
	Pattern      string     `gorm:"type:text"`
	KeywordsJSON string     `gorm:"type:json"`
	TargetField  string     `gorm:"size:32;not null"`
	EdgePred     string     `gorm:"size:32;not null;index"`
	Severity     int        `gorm:"not null"`
	Confidence   float64    `gorm:"not null"`
	EvidenceJSON string     `gorm:"type:json"`
	AdviceCode   string     `gorm:"size:128"`
	Advice       string     `gorm:"type:text"`
	Priority     int        `gorm:"not null;index"`
	Enabled      bool       `gorm:"not null;index"`
	Status       string     `gorm:"size:32;not null;index"`
	CreatedAt    time.Time  `gorm:"autoCreateTime"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime"`
	DeletedAt    *time.Time `gorm:"index"`
}

func (EdgeRuleModel) TableName() string { return "edge_rules" }

type EdgeTemplateModel struct {
	ID           int64      `gorm:"primaryKey;autoIncrement"`
	TemplateID   string     `gorm:"size:64;not null;index"`
	TemplateText string     `gorm:"type:text;not null"`
	RawExample   string     `gorm:"type:text"`
	Source       string     `gorm:"size:64;not null"`
	EdgePred     string     `gorm:"size:32;not null;index"`
	Severity     int        `gorm:"not null"`
	Confidence   float64    `gorm:"not null"`
	EvidenceJSON string     `gorm:"type:json"`
	AdviceCode   string     `gorm:"size:128"`
	Advice       string     `gorm:"type:text"`
	Status       string     `gorm:"size:32;not null;index"`
	CreatedAt    time.Time  `gorm:"autoCreateTime"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime"`
	DeletedAt    *time.Time `gorm:"index"`
}

func (EdgeTemplateModel) TableName() string { return "edge_templates" }

type EdgeRuleSetModel struct {
	ID            int64     `gorm:"primaryKey;autoIncrement"`
	Name          string    `gorm:"size:255;not null"`
	Description   string    `gorm:"type:text"`
	Status        string    `gorm:"size:32;not null;index"`
	ConfigVersion int64     `gorm:"not null;index"`
	Etag          string    `gorm:"size:255"`
	ItemIDsJSON   string    `gorm:"type:json"`
	ItemsJSON     string    `gorm:"type:json"`
	ItemCount     int       `gorm:"not null"`
	CreatedAt     time.Time `gorm:"autoCreateTime"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime"`
}

func (EdgeRuleSetModel) TableName() string { return "edge_rule_sets" }

type EdgeTemplateSetModel struct {
	ID            int64     `gorm:"primaryKey;autoIncrement"`
	Name          string    `gorm:"size:255;not null"`
	Description   string    `gorm:"type:text"`
	Status        string    `gorm:"size:32;not null;index"`
	ConfigVersion int64     `gorm:"not null;index"`
	Etag          string    `gorm:"size:255"`
	ItemIDsJSON   string    `gorm:"type:json"`
	ItemsJSON     string    `gorm:"type:json"`
	ItemCount     int       `gorm:"not null"`
	CreatedAt     time.Time `gorm:"autoCreateTime"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime"`
}

func (EdgeTemplateSetModel) TableName() string { return "edge_template_sets" }
