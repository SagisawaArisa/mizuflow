package repository

import (
	"context"
	"errors"
	"mizuflow/internal/model"

	"gorm.io/gorm"
)

// SDKRepository defines the interface for SDK Key validation
type SDKRepository interface {
	ValidateAPIKey(ctx context.Context, apiKey, env string) (bool, error)
}

// SDKKeyRepository implementation
type SDKKeyRepository struct {
	db *gorm.DB
}

func NewSDKKeyRepository(db *gorm.DB) *SDKKeyRepository {
	return &SDKKeyRepository{db: db}
}

func (r *SDKKeyRepository) ValidateAPIKey(ctx context.Context, apiKey, env string) (bool, error) {
	// TODO: Add caching layer (Dogfooding etcd watch /Memory) here
	var client model.SDKClient
	err := r.db.WithContext(ctx).
		Where("api_key = ? AND env = ? AND status = 1", apiKey, env).
		First(&client).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
