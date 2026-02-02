package service

import (
	"context"
	"mizuflow/internal/model"
	"mizuflow/internal/repository"
	v1 "mizuflow/pkg/api/v1"
	"mizuflow/pkg/logger"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.uber.org/zap"
)

type ReconcilerConfig struct {
	Interval   time.Duration
	BatchSize  int
	BatchDelay time.Duration
}

type Reconciler struct {
	etcdClient  *clientv3.Client
	etcdRepo    *repository.FeatureRepository
	featureRepo repository.FeatureInterface
	config      ReconcilerConfig
}

func NewReconciler(client *clientv3.Client, etcdRepo *repository.FeatureRepository, featureRepo repository.FeatureInterface, cfg ReconcilerConfig) *Reconciler {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	return &Reconciler{
		etcdClient:  client,
		etcdRepo:    etcdRepo,
		featureRepo: featureRepo,
		config:      cfg,
	}
}

func (r *Reconciler) Run(ctx context.Context) {
	ticker := time.NewTicker(r.config.Interval)
	defer ticker.Stop()

	// Session for distributed lock, tightly coupled with a lease
	session, err := concurrency.NewSession(r.etcdClient, concurrency.WithTTL(10))
	if err != nil {
		logger.Error("failed to create etcd concurrency session", zap.Error(err))
		return
	}
	defer session.Close()

	mutex := concurrency.NewMutex(session, "/locks/reconciler")

	logger.Info("reconciler started", zap.Duration("interval", r.config.Interval))

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Try to acquire distributed lock
			lockCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := mutex.Lock(lockCtx)
			cancel()

			if err != nil {
				if err == context.DeadlineExceeded {
					// Lock held by another instance, skip this round
					logger.Debug("reconciliation skipped, another instance holds the lock")
				} else {
					logger.Error("failed to acquire reconciliation lock", zap.Error(err))
				}
				continue
			}

			// Lock acquired, run the job
			logger.Info("lock acquired, starting reconciliation")
			r.reconcile(ctx)

			// Release lock
			if err := mutex.Unlock(context.Background()); err != nil {
				logger.Warn("failed to release reconciliation lock", zap.Error(err))
			}
		}
	}
}

func (r *Reconciler) reconcile(ctx context.Context) {
	batchSize := r.config.BatchSize
	offset := 0
	for {
		// TODO: Circuit Breaker for DB Outage
		// In a disaster scenario where MySQL is down, the Reconciler MUST NOT overwrite Etcd data.
		// If db read fails, we should abort the reconciliation process immediately to preserve
		// any emergency flags manually set in Etcd (which might be the source of truth during an outage).
		dbFeatures, err := r.featureRepo.ListByPage(ctx, offset, batchSize)
		if err != nil {
			logger.Error("recon: failed to fetch features from db, aborting to prevent unsafe overwrite", zap.Error(err))
			return
		}
		if len(dbFeatures) == 0 {
			break
		}
		for _, dbItem := range dbFeatures {
			r.checkOne(ctx, dbItem)
		}
		offset += batchSize
		logger.Info("reconciliation batch processed", zap.Int("batch_size", len(dbFeatures)), zap.Int("next_offset", offset))

		if r.config.BatchDelay > 0 {
			time.Sleep(r.config.BatchDelay)
		}
	}

	logger.Info("reconciliation finished")
}

func (r *Reconciler) checkOne(ctx context.Context, dbItem *model.FeatureMaster) {
	fullKey := BuildFeatureKey(dbItem.Env, dbItem.Namespace, dbItem.Key)
	etcdFlag, err := r.etcdRepo.GetFeature(ctx, fullKey)
	if err != nil {
		logger.Error("recon: failed to get feature from etcd", zap.String("key", fullKey), zap.Error(err))
		return
	}
	shouldFix := false
	if etcdFlag == nil {
		shouldFix = true
	} else if etcdFlag.Version < dbItem.Version {
		shouldFix = true
	}

	if shouldFix {
		logger.Warn("recon: fixing single inconsistency", zap.String("key", fullKey), zap.Int("db_version", dbItem.Version), zap.Int("etcd_version", func() int {
			if etcdFlag == nil {
				return 0
			}
			return etcdFlag.Version
		}()))

		// Construct payload
		flag := v1.FeatureFlag{
			Namespace: dbItem.Namespace,
			Env:       dbItem.Env,
			Key:       dbItem.Key,
			Value:     dbItem.CurrentVal,
			Type:      dbItem.Type,
			Version:   dbItem.Version,
		}

		_, err := r.etcdRepo.SaveFeatureIfNewer(ctx, fullKey, flag)
		if err != nil {
			logger.Error("recon: failed to fix etcd", zap.String("key", fullKey), zap.Error(err))
		}
	}
}
