package model

import "time"

type OutboxTask struct {
	ID         int64  `json:"id" gorm:"primaryKey"`
	Key        string `json:"key" gorm:"size:128;index"`
	Payload    string `json:"payload" gorm:"type:text"`
	Status     int    `json:"status" gorm:"index"`
	RetryCount int    `json:"retry_count" gorm:"default:0"`
	TraceID    string `json:"trace_id" gorm:"size:64;index"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

const (
	StatusPending   = 0
	StatusCompleted = 1
	StatusFailed    = 2
)
