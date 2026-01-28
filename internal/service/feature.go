package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mizuflow/internal/buffer"
	"mizuflow/internal/model"
	"mizuflow/internal/repository"
	v1 "mizuflow/pkg/api/v1"
	"mizuflow/pkg/constraints"
	"strings"

	"mizuflow/internal/dto/resp"

	"mizuflow/pkg/logger"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var ErrAuditNotMatch = errors.New("audit record key mismatch")
var ErrEtcdUnhealthy = errors.New("etcd unhealthy")
var ErrMysqlUnhealthy = errors.New("mysql unhealthy")

const FeatureRootPrefix = "/mizuflow/"

func BuildFeatureKey(env, namespace, key string) string {
	return fmt.Sprintf("%s%s/%s/features/%s", FeatureRootPrefix, env, namespace, key)
}

type FeatureService struct {
	db          *gorm.DB
	etcdRepo    *repository.FeatureRepository
	auditRepo   repository.AuditInterface
	featureRepo repository.FeatureInterface
	outboxRepo  repository.OutboxInterface
	buffer      *buffer.RevisionBuffer
	cache       *FeatureCache
	hub         *Hub
}

type Transactional interface {
	WithTx(tx *gorm.DB) any
}

func NewFeatureService(db *gorm.DB, etcdRepo *repository.FeatureRepository, mysqlRepo repository.AuditInterface, featureRepo repository.FeatureInterface, outboxRepo repository.OutboxInterface, hub *Hub) *FeatureService {
	return &FeatureService{
		db:          db,
		etcdRepo:    etcdRepo,
		auditRepo:   mysqlRepo,
		featureRepo: featureRepo,
		outboxRepo:  outboxRepo,
		hub:         hub,
		buffer:      buffer.NewRevisionBuffer(1000),
		cache:       NewFeatureCache(),
	}
}

func (s *FeatureService) GetCompensation(lastRev int64) ([]v1.Message, bool) {
	return s.buffer.GetSince(lastRev)
}

func (s *FeatureService) SaveFeature(ctx context.Context, flag v1.FeatureFlag, operator string) (int, error) {
	var lastestVersion int
	var outboxID uint64
	// todo replacement for traceID
	traceID, _ := ctx.Value("TraceID").(string)

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txFeature := s.featureRepo.WithTx(tx).(repository.FeatureInterface)
		txAudit := s.auditRepo.WithTx(tx).(repository.AuditInterface)
		txOutbox := s.outboxRepo.WithTx(tx).(repository.OutboxInterface)

		// maintain master record
		master, err := txFeature.GetByKey(ctx, flag.Namespace, flag.Env, flag.Key)

		if err != nil {
			logger.Error("failed to get feature master", zap.String("key", flag.Key), zap.Error(err))
			return err
		}
		var oldValue string

		if master == nil {
			master = &model.FeatureMaster{
				Namespace:  flag.Namespace,
				Env:        flag.Env,
				Key:        flag.Key,
				Type:       flag.Type,
				Version:    1,
				CurrentVal: flag.Value,
			}
		} else {
			oldValue = master.CurrentVal
			master.Version++
			master.CurrentVal = flag.Value
			master.Type = flag.Type
		}
		txFeature.Save(ctx, master)
		lastestVersion = master.Version
		// record audit logging
		audit := &model.FeatureAudit{
			Namespace: flag.Namespace,
			Env:       flag.Env,
			Key:       flag.Key,
			OldValue:  oldValue,
			NewValue:  flag.Value,
			Type:      flag.Type,
			Operator:  operator,
			TraceID:   traceID,
		}
		if err := txAudit.Create(ctx, audit); err != nil {
			logger.Error("failed to create feature audit", zap.String("key", flag.Key), zap.Error(err))
			return err
		}

		// create outbox event
		flag.Version = lastestVersion
		pBytes, _ := json.Marshal(flag)
		event := &model.OutboxTask{
			Key:     flag.Key,
			Payload: string(pBytes),
			Status:  model.StatusPending,
			TraceID: traceID,
		}
		if err := txOutbox.Create(ctx, event); err != nil {
			logger.Error("failed to create outbox event", zap.String("key", flag.Key), zap.Error(err))
			return err
		}
		outboxID = uint64(event.ID)

		return nil
	})

	if err != nil {
		return 0, errors.New("feature save failed")
	}

	go s.syncToEtcd(outboxID, flag)
	return lastestVersion, nil
}

func (s *FeatureService) syncToEtcd(outboxID uint64, flag v1.FeatureFlag) {
	// construct key with namespace and env
	// e.g., /mizuflow/dev/default/features/my-feature
	fullKey := BuildFeatureKey(flag.Env, flag.Namespace, flag.Key)
	_, err := s.etcdRepo.SaveFeatureIfNewer(context.Background(), fullKey, flag)
	if err != nil {
		logger.Warn("failed to sync feature to etcd", zap.String("key", flag.Key), zap.Error(err))
		return
	}
	_ = s.outboxRepo.UpdateStatus(context.Background(), outboxID, model.StatusCompleted, 0)
}

func (s *FeatureService) GetFeature(ctx context.Context, namespace, env, key string) (*resp.FeatureItem, error) {
	m, err := s.featureRepo.GetByKey(ctx, namespace, env, key)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, errors.New("feature not found")
	}

	return &resp.FeatureItem{
		ID:        m.ID,
		Namespace: m.Namespace,
		Env:       m.Env,
		Key:       m.Key,
		Type:      m.Type,
		Version:   m.Version,
		Value:     m.CurrentVal,
		UpdatedAt: m.UpdatedAt,
		UpdatedBy: m.UpdatedBy,
	}, nil
}

func (s *FeatureService) ListFeatures(ctx context.Context, namespace, env, search string) ([]resp.FeatureItem, error) {
	masters, err := s.featureRepo.List(ctx, namespace, env, search)
	if err != nil {
		return nil, err
	}
	items := make([]resp.FeatureItem, 0, len(masters))
	for _, m := range masters {
		items = append(items, resp.FeatureItem{
			ID:        m.ID,
			Namespace: m.Namespace,
			Env:       m.Env,
			Key:       m.Key,
			Type:      m.Type,
			Version:   m.Version,
			Value:     m.CurrentVal,
			UpdatedAt: m.UpdatedAt,
			UpdatedBy: m.UpdatedBy,
		})
	}
	return items, nil
}

func (s *FeatureService) GetFeatureAudits(ctx context.Context, namespace, env, key string) ([]resp.AuditLogItem, error) {
	audits, err := s.auditRepo.ListByKey(ctx, namespace, env, key)
	if err != nil {
		return nil, err
	}
	items := make([]resp.AuditLogItem, 0, len(audits))
	for _, a := range audits {
		items = append(items, resp.AuditLogItem{
			ID:        a.ID,
			Namespace: a.Namespace,
			Env:       a.Env,
			Key:       a.Key,
			OldValue:  a.OldValue,
			NewValue:  a.NewValue,
			Type:      a.Type,
			Operator:  a.Operator,
			CreatedAt: a.CreatedAt,
		})
	}
	return items, nil
}

func (s *FeatureService) GetAllFeatures(ctx context.Context) ([]v1.FeatureFlag, int64) {
	snapshot, rev := s.cache.GetSnapshot()

	return snapshot, rev
}

func (s *FeatureService) RollbackFeature(ctx context.Context, namespace, env, key string, auditID uint, operator string) (int, error) {
	audit, err := s.auditRepo.FindByID(ctx, auditID)
	if err != nil {
		return 0, err
	}
	// Security Check: Ensure the audit log belongs to the context we are operating on
	if audit.Key != key || audit.Env != env || audit.Namespace != namespace {
		return 0, fmt.Errorf("audit record mismatch: valid for %s/%s/%s only", audit.Env, audit.Namespace, audit.Key)
	}

	// Fetch current version for CAS protection
	master, err := s.featureRepo.GetByKey(ctx, namespace, env, key)
	if err != nil {
		return 0, err
	}
	var currentVersion int
	if master != nil {
		currentVersion = master.Version
	}

	logger.Info("rolling back feature", zap.String("key", key), zap.String("from_val", audit.NewValue), zap.String("to_val", audit.OldValue))

	return s.SaveFeature(ctx, v1.FeatureFlag{
		Namespace: namespace,
		Env:       env,
		Key:       key,
		Value:     audit.OldValue,
		Type:      audit.Type,
		Version:   currentVersion, // Use current version to enforce CAS
	}, operator)
}

func (s *FeatureService) Health(ctx context.Context) error {
	if s.auditRepo.PingContext(ctx) != nil {
		return ErrMysqlUnhealthy
	}
	if s.etcdRepo.Health(ctx) != nil {
		return ErrEtcdUnhealthy
	}
	return nil
}

func (s *FeatureService) Run(ctx context.Context) {
	prefix := FeatureRootPrefix
	// get initial snapshot
	resp, err := s.etcdRepo.GetWithRevision(ctx, prefix)
	if err != nil {
		logger.Error("failed to get initial features", zap.Error(err))
		return
	}
	// avoid missing updates between Get and Watch
	rev0 := resp.Header.Revision
	for _, kv := range resp.Kvs {
		var flag v1.FeatureFlag
		if err := json.Unmarshal(kv.Value, &flag); err != nil {
			logger.Warn("failed to unmarshal feature during snapshot", zap.String("key", string(kv.Key)))
			continue
		}
		flag.Revision = kv.ModRevision
		s.cache.Update(flag)
	}
	logger.Info("feature snapshot initialized", zap.Int64("rev", rev0))

	watchChan := s.etcdRepo.WatchFeatureFrom(ctx, prefix, rev0+1)
	for {
		select {
		case <-ctx.Done():
			return
		case wresp := <-watchChan:
			if wresp.Canceled {
				// TODO rebuild
				logger.Warn("watch canceled", zap.Error(wresp.Err()))
				return
			}
			for _, ev := range wresp.Events {
				// update snapshot
				var msg v1.Message
				if ev.Type == clientv3.EventTypeDelete {
					// Extract metadata from key: /mizuflow/:env/:ns/features/:key
					// Since delete event doesn't have value, we must parse the key
					var env, ns, key string
					// Simplified parser compatible with BuildFeatureKey
					// Key structure: /mizuflow/{env}/{namespace}/features/{key}
					// example: /mizuflow/dev/default/features/my-key
					parts := strings.Split(string(ev.Kv.Key), "/")
					if len(parts) >= 6 {
						env = parts[2]
						ns = parts[3]
						key = parts[5]
					} else {
						// fallback
						key = string(ev.Kv.Key)
					}

					msg = v1.Message{
						Namespace: ns,
						Env:       env,
						Key:       key,
						Revision:  ev.Kv.ModRevision,
						Action:    constraints.DELETE,
					}
					s.cache.Delete(string(ev.Kv.Key), ev.Kv.ModRevision)
				} else {
					var flag v1.FeatureFlag
					if err := json.Unmarshal(ev.Kv.Value, &flag); err != nil {
						logger.Error("failed to unmarshal feature flag", zap.String("key", msg.Key), zap.ByteString("raw_value", ev.Kv.Value))
						continue
					}
					msg = v1.Message{
						Namespace: flag.Namespace,
						Env:       flag.Env,
						Key:       flag.Key,
						Value:     flag.Value,
						Type:      flag.Type,
						Version:   flag.Version,
						Revision:  ev.Kv.ModRevision,
						Action:    constraints.PUT,
					}
					s.cache.Update(flag)
				}
				// update buffer
				s.buffer.AddMessage(msg)
				// broadcast to clients
				s.hub.Broadcast <- msg
			}
		}
	}
}
