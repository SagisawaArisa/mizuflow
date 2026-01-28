package model

import "time"

type FeatureAudit struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	Namespace string    `json:"namespace" gorm:"default:default"`
	Env       string    `json:"env" gorm:"default:dev"`
	Key       string    `json:"key" gorm:"size:128;index"`
	OldValue  string    `json:"old_value" gorm:"type:text"`
	NewValue  string    `json:"new_value" gorm:"type:text"`
	Type      string    `json:"type" gorm:"size:32"`
	Operator  string    `json:"operator" gorm:"size:64"`
	TraceID   string    `json:"trace_id" gorm:"size:36;index"`
	IP        string    `json:"ip" gorm:"size:45"`
	CreatedAt time.Time `json:"created_at" gorm:"index"`
}
