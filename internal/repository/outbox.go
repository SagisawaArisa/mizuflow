package repository

import (
	"context"
	"mizuflow/internal/model"

	"gorm.io/gorm"
)

type OutboxInterface interface {
	Create(ctx context.Context, outbox *model.OutboxTask) error
	FetchPending(ctx context.Context, limit int) ([]model.OutboxTask, error)
	UpdateStatus(ctx context.Context, id uint64, status int, retryCount int) error
	WithTx(tx *gorm.DB) OutboxInterface
}

type OutboxRepository struct {
	db *gorm.DB
}

func NewOutboxRepository(db *gorm.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

func (r *OutboxRepository) Create(ctx context.Context, outbox *model.OutboxTask) error {
	return r.db.WithContext(ctx).Create(outbox).Error
}

func (r *OutboxRepository) FetchPending(ctx context.Context, limit int) ([]model.OutboxTask, error) {
	var tasks []model.OutboxTask
	// only fetch tasks with status 'pending'
	if err := r.db.WithContext(ctx).Where("status = ?", model.StatusPending).
		Limit(limit).Order("id ASC").Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}
func (r *OutboxRepository) UpdateStatus(ctx context.Context, id uint64, status int, retryCount int) error {
	return r.db.WithContext(ctx).Model(&model.OutboxTask{}).Where("id = ?", id).Updates(map[string]any{
		"status":      status,
		"retry_count": retryCount,
	}).Error
}
func (r *OutboxRepository) WithTx(tx *gorm.DB) OutboxInterface {
	return &OutboxRepository{db: tx}
}
