package service

import (
	"context"
	"delta-conf/internal/repository"

	"delta-conf/pkg/logger"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

type ConfigService struct {
	repo   *repository.ConfigRepository
	buffer *RevisionBuffer
	cache  *ConfigCache
	hub    *Hub
}

func NewConfigService(repo *repository.ConfigRepository, hub *Hub) *ConfigService {
	return &ConfigService{
		repo:   repo,
		hub:    hub,
		buffer: NewRevisionBuffer(1000),
		cache:  NewConfigCache(),
	}
}

func (s *ConfigService) StartWatch(ctx context.Context) {
	logger.Info("start watching configs...")
	watchChan := s.repo.WatchConfig(ctx, "/configs/")

	for {
		select {
		case <-ctx.Done():
			logger.Info("stopping config watch...")
			return
		case resp := <-watchChan:
			for _, ev := range resp.Events {
				msg := Message{
					Revision: ev.Kv.ModRevision,
					Key:      string(ev.Kv.Key),
					Value:    string(ev.Kv.Value),
				}
				s.hub.Broadcast <- msg

				logger.Info("config changed and notification sent",
					zap.String("key", msg.Key),
					zap.Int64("rev", msg.Revision),
					zap.String("type", ev.Type.String()))
			}
		}
	}
}

func (s *ConfigService) GetCompensation(lastRev int64) ([]Message, bool) {
	return s.buffer.GetSince(lastRev)
}

func (s *ConfigService) Run(ctx context.Context) {
	prefix := "/configs/"
	resp, err := s.repo.GetWithRevision(ctx, prefix)
	if err != nil {
		logger.Error("failed to get initial configs", zap.Error(err))
		return
	}
	rev0 := resp.Header.Revision
	for _, kv := range resp.Kvs {
		s.cache.Update(string(kv.Key), string(kv.Value), kv.ModRevision)
	}
	logger.Info("config snapshot initialized", zap.Int64("rev", rev0))

	watchChan := s.repo.WatchConfigFrom(ctx, prefix, rev0+1)
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
				msg := Message{
					Revision: ev.Kv.ModRevision,
					Key:      string(ev.Kv.Key),
					Value:    string(ev.Kv.Value),
				}
				// update snapshot
				if ev.Type == clientv3.EventTypeDelete {
					s.cache.Delete(msg.Key, msg.Revision)
				} else {
					s.cache.Update(msg.Key, msg.Value, msg.Revision)
				}
				// update buffer
				s.buffer.AddMessage(msg)
				// broadcast to clients
				s.hub.Broadcast <- msg
			}
		}
	}
}
