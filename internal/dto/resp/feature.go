package resp

import (
	"time"

	v1 "mizuflow/pkg/api/v1"
)

type CreateFeatureResponse struct {
	Version int `json:"version"`
}

type GetFeatureResponse struct {
	*FeatureItem
}

type RollbackFeatureResponse struct {
	Version int `json:"version"`
}

type SnapshotResponse struct {
	Data     []v1.FeatureFlag `json:"data"`
	Revision int64            `json:"revision"`
}

type FeatureItem struct {
	ID        uint64    `json:"id"`
	Namespace string    `json:"namespace"`
	Env       string    `json:"env"`
	Key       string    `json:"key"`
	Type      string    `json:"type"`
	Version   int       `json:"version"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy string    `json:"updated_by"`
}

type AuditLogItem struct {
	ID        int64     `json:"id"`
	Namespace string    `json:"namespace"`
	Env       string    `json:"env"`
	Key       string    `json:"key"`
	OldValue  string    `json:"old_value"`
	NewValue  string    `json:"new_value"`
	Type      string    `json:"type"`
	Operator  string    `json:"operator"`
	CreatedAt time.Time `json:"created_at"`
}
