package repository

import (
	"context"
	"errors"
	"mizuflow/internal/model"

	"gorm.io/gorm"
)

// FeatureInterface defines the interface for feature master data persistence
type FeatureInterface interface {
	GetByKey(ctx context.Context, namespace, env, key string) (*model.FeatureMaster, error)
	GetAll(ctx context.Context) ([]*model.FeatureMaster, error)
	List(ctx context.Context, namespace, env, search string) ([]*model.FeatureMaster, error)
	Save(ctx context.Context, master *model.FeatureMaster) error
	Rollback(ctx context.Context, namespace, env, key string, version int) (*model.FeatureMaster, error)
	WithTx(tx *gorm.DB) any
}

// FeatureMasterRepository implementation of FeatureInterface for MySQL
type FeatureMasterRepository struct {
	db *gorm.DB
}

// NewFeatureMasterRepository creates a new instance
func NewFeatureMasterRepository(db *gorm.DB) *FeatureMasterRepository {
	return &FeatureMasterRepository{db: db}
}

// GetByKey retrieves the feature master record by its key
func (r *FeatureMasterRepository) GetByKey(ctx context.Context, namespace, env, key string) (*model.FeatureMaster, error) {
	var feature model.FeatureMaster
	if err := r.db.WithContext(ctx).Where("namespace = ? AND env = ? AND `key` = ?", namespace, env, key).First(&feature).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &feature, nil
}

func (r *FeatureMasterRepository) GetAll(ctx context.Context) ([]*model.FeatureMaster, error) {
	var features []*model.FeatureMaster
	err := r.db.WithContext(ctx).Find(&features).Error
	return features, err
}

func (r *FeatureMasterRepository) List(ctx context.Context, namespace, env, search string) ([]*model.FeatureMaster, error) {
	var features []*model.FeatureMaster
	query := r.db.WithContext(ctx)

	if namespace != "" {
		query = query.Where("namespace = ?", namespace)
	}
	if env != "" {
		query = query.Where("env = ?", env)
	}
	if search != "" {
		query = query.Where("`key` LIKE ?", "%"+search+"%")
	}

	err := query.Order("updated_at DESC").Find(&features).Error
	return features, err
}

// Save creates or updates the feature master record
func (r *FeatureMasterRepository) Save(ctx context.Context, master *model.FeatureMaster) error {
	return r.db.WithContext(ctx).Save(master).Error
}

// Rollback restores a feature version. Here 'version' is interpreted as the Audit ID to restore from.
func (r *FeatureMasterRepository) Rollback(ctx context.Context, namespace, env, key string, version int) (*model.FeatureMaster, error) {
	// Find the audit log corresponding to the version (Assuming version matches Audit ID)
	var audit model.FeatureAudit
	if err := r.db.WithContext(ctx).First(&audit, version).Error; err != nil {
		return nil, err
	}

	if audit.Key != key {
		return nil, errors.New("audit record key mismatch")
	}

	// Retrieve current master to update it
	master, err := r.GetByKey(ctx, namespace, env, key)
	if err != nil {
		return nil, err
	}
	if master == nil {
		// If master doesn't exist, we recreate it from audit
		master = &model.FeatureMaster{
			Namespace: namespace,
			Env:       env,
			Key:       key,
		}
	}

	// Restore state from audit
	master.CurrentVal = audit.NewValue
	master.Type = audit.Type
	// Increment version in master to indicate a new change (the rollback itself is a change)
	master.Version = master.Version + 1

	if err := r.Save(ctx, master); err != nil {
		return nil, err
	}

	return master, nil
}

func (r *FeatureMasterRepository) WithTx(tx *gorm.DB) any {
	return &FeatureMasterRepository{db: tx}
}
