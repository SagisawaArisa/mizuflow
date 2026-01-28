package repository

import (
	"context"
	"mizuflow/internal/model"

	"gorm.io/gorm"
)

// AuditInterface defines the interface for audit log persistence
type AuditInterface interface {
	Create(ctx context.Context, audit *model.FeatureAudit) error
	FindByID(ctx context.Context, id uint) (*model.FeatureAudit, error)
	List(ctx context.Context, offset, limit int) ([]model.FeatureAudit, int64, error)
	ListByKey(ctx context.Context, namespace, env, key string) ([]model.FeatureAudit, error)
	PingContext(ctx context.Context) error
	WithTx(tx *gorm.DB) any
}

// AuditRepository is the domain repository that wraps the storage
type AuditRepository struct {
	db *gorm.DB
}

// NewAuditRepository creates a new instance of AuditRepository
func NewAuditRepository(db *gorm.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) Create(ctx context.Context, audit *model.FeatureAudit) error {
	return r.db.WithContext(ctx).Create(audit).Error
}

func (r *AuditRepository) FindByID(ctx context.Context, id uint) (*model.FeatureAudit, error) {
	var audit model.FeatureAudit
	if err := r.db.WithContext(ctx).First(&audit, id).Error; err != nil {
		return nil, err
	}
	return &audit, nil
}

func (r *AuditRepository) List(ctx context.Context, offset, limit int) ([]model.FeatureAudit, int64, error) {
	var audits []model.FeatureAudit
	var total int64

	db := r.db.WithContext(ctx).Model(&model.FeatureAudit{})
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := db.Offset(offset).Limit(limit).Order("id DESC").Find(&audits).Error; err != nil {
		return nil, 0, err
	}

	return audits, total, nil
}

func (r *AuditRepository) ListByKey(ctx context.Context, namespace, env, key string) ([]model.FeatureAudit, error) {
	var audits []model.FeatureAudit
	err := r.db.WithContext(ctx).
		Where("namespace = ? AND env = ? AND `key` = ?", namespace, env, key).
		Order("created_at DESC").
		Find(&audits).Error
	return audits, err
}

func (r *AuditRepository) WithTx(tx *gorm.DB) any {
	return &AuditRepository{db: tx}
}

func (r *AuditRepository) PingContext(ctx context.Context) error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}
