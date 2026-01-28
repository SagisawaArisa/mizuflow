package service

import (
	"context"
	"encoding/json"
	"mizuflow/internal/model"
	"mizuflow/internal/repository"
	v1 "mizuflow/pkg/api/v1"
	"mizuflow/pkg/logger"
	"time"

	"go.uber.org/zap"
)

type OutboxWorker struct {
	outboxRepo repository.OutboxInterface
	etcdRepo   *repository.FeatureRepository
	interval   time.Duration
}

func NewOutboxWorker(outboxRepo repository.OutboxInterface, etcdRepo *repository.FeatureRepository, interval time.Duration) *OutboxWorker {
	return &OutboxWorker{
		outboxRepo: outboxRepo,
		etcdRepo:   etcdRepo,
		interval:   interval,
	}
}

func (w *OutboxWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	logger.Info("outbox worker started", zap.Duration("interval", w.interval))

	for {
		select {
		case <-ctx.Done():
			logger.Info("outbox worker stopped")
			return
		case <-ticker.C:
			w.processPending(ctx)
		}
	}
}

func (w *OutboxWorker) processPending(ctx context.Context) {
	// Fetch batch of pending tasks
	tasks, err := w.outboxRepo.FetchPending(ctx, 10)
	if err != nil {
		logger.Error("failed to fetch pending outbox tasks", zap.Error(err))
		return
	}

	for _, task := range tasks {
		logger.Debug("processing outbox task", zap.Int64("id", task.ID), zap.String("key", task.Key))

		var flag v1.FeatureFlag
		// Payload is the JSON string of feature flag
		if err := json.Unmarshal([]byte(task.Payload), &flag); err != nil {
			logger.Error("failed to unmarshal task payload", zap.Int64("id", task.ID), zap.Error(err))
			// Mark as failed directly since payload is corrupt
			w.outboxRepo.UpdateStatus(ctx, uint64(task.ID), model.StatusFailed, task.RetryCount)
			continue
		}

		// Sync to Etcd
		fullKey := BuildFeatureKey(flag.Env, flag.Namespace, flag.Key)
		_, err := w.etcdRepo.SaveFeatureIfNewer(ctx, fullKey, flag)
		if err != nil {
			logger.Warn("failed to sync task to etcd", zap.Int64("id", task.ID), zap.Error(err))
			newRetryCount := task.RetryCount + 1
			if newRetryCount >= 5 {
				logger.Error("task max retries reached", zap.Int64("id", task.ID))
				w.outboxRepo.UpdateStatus(ctx, uint64(task.ID), model.StatusFailed, newRetryCount)
			} else {
				w.outboxRepo.UpdateStatus(ctx, uint64(task.ID), model.StatusPending, newRetryCount)
			}
			continue
		}

		// Success
		if err := w.outboxRepo.UpdateStatus(ctx, uint64(task.ID), model.StatusCompleted, task.RetryCount); err != nil {
			logger.Error("failed to mark task completed", zap.Int64("id", task.ID), zap.Error(err))
		} else {
			logger.Info("outbox task completed", zap.Int64("id", task.ID), zap.String("key", task.Key))
		}
	}
}
