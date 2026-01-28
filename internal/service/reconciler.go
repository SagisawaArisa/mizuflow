package service

import (
	"context"
	"encoding/json"
	"mizuflow/internal/model"
	"mizuflow/internal/repository"
	v1 "mizuflow/pkg/api/v1"
	"mizuflow/pkg/logger"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.uber.org/zap"
)

type Reconciler struct {
	etcdClient  *clientv3.Client
	etcdRepo    *repository.FeatureRepository
	featureRepo repository.FeatureInterface
	interval    time.Duration
}

func NewReconciler(client *clientv3.Client, etcdRepo *repository.FeatureRepository, featureRepo repository.FeatureInterface, interval time.Duration) *Reconciler {
	return &Reconciler{
		etcdClient:  client,
		etcdRepo:    etcdRepo,
		featureRepo: featureRepo,
		interval:    interval,
	}
}

func (r *Reconciler) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	// Session for distributed lock, tightly coupled with a lease
	session, err := concurrency.NewSession(r.etcdClient, concurrency.WithTTL(10))
	if err != nil {
		logger.Error("failed to create etcd concurrency session", zap.Error(err))
		return
	}
	defer session.Close()

	mutex := concurrency.NewMutex(session, "/locks/reconciler")

	logger.Info("reconciler started", zap.Duration("interval", r.interval))

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
	// todo batching/cursor
	// Get all from MySQL
	dbFeatures, err := r.featureRepo.GetAll(ctx)
	if err != nil {
		logger.Error("recon: failed to fetch features from db", zap.Error(err))
		return
	}
	dbMap := make(map[string]*model.FeatureMaster)
	for _, f := range dbFeatures {
		copy := f
		fullKey := BuildFeatureKey(f.Env, f.Namespace, f.Key)
		dbMap[fullKey] = copy
	}

	// Get all from Etcd
	resp, err := r.etcdRepo.GetWithRevision(ctx, FeatureRootPrefix)
	if err != nil {
		logger.Error("recon: failed to fetch features from etcd", zap.Error(err))
		return
	}
	etcdMap := make(map[string]*v1.FeatureFlag)
	for _, kv := range resp.Kvs {
		var flag v1.FeatureFlag
		if err := json.Unmarshal(kv.Value, &flag); err == nil {
			etcdMap[string(kv.Key)] = &flag
		}
	}

	// MySQL has, Etcd missing/old
	for fullKey, dbModel := range dbMap {
		// Etcd keys stored as full paths, check logic if keys match
		etcdFlag, exists := etcdMap[fullKey]

		shouldUpdate := false
		reason := ""

		if !exists {
			shouldUpdate = true
			reason = "missing_in_etcd"
		} else {
			// Compare Content.
			if dbModel.CurrentVal != etcdFlag.Value || dbModel.Type != etcdFlag.Type {
				shouldUpdate = true
				reason = "value_mismatch"
			}
		}

		if shouldUpdate {
			logger.Warn("recon: fixing inconsistency", zap.String("key", fullKey), zap.String("reason", reason))

			// Construct payload
			flag := v1.FeatureFlag{
				Namespace: dbModel.Namespace,
				Env:       dbModel.Env,
				Key:       dbModel.Key,
				Value:     dbModel.CurrentVal,
				Type:      dbModel.Type,
				Version:   dbModel.Version, // Correctly set Business Version
			}

			_, err := r.etcdRepo.SaveFeatureIfNewer(ctx, fullKey, flag)
			if err != nil {
				logger.Error("recon: failed to fix etcd", zap.String("key", fullKey), zap.Error(err))
			}
		}
	}

	// Etcd has, MySQL missing
	for fullKey := range etcdMap {
		if _, exists := dbMap[fullKey]; !exists {
			logger.Warn("recon: removing orphan key", zap.String("key", fullKey))
		}
	}

	logger.Info("reconciliation finished", zap.Int("db_count", len(dbMap)), zap.Int("etcd_count", len(etcdMap)))
}
