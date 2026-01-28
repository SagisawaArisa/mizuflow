package model

import "time"

type FeatureMaster struct {
	ID         uint64    `gorm:"primaryKey" json:"id"`
	Namespace  string    `gorm:"default:default" json:"namespace"`
	Env        string    `gorm:"default:dev" json:"env"`
	Key        string    `json:"key"`
	Type       string    `json:"type"`
	Version    int       `json:"version"`
	CurrentVal string    `json:"value"`
	UpdatedAt  time.Time `json:"updated_at"`
	UpdatedBy  string    `json:"updated_by"` // derived
}
