package repository

import (
	"context"
	"delta-conf/internal/model"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type ConfigRepository struct {
	client *clientv3.Client
}

func NewConfigRepository(endpoints []string) (*ConfigRepository, error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	return &ConfigRepository{
		client: client,
	}, nil
}

// GetConfig retrieves a configuration item by key from etcd.
func (r *ConfigRepository) GetConfig(ctx context.Context, key string) (*model.ConfigItem, error) {
	resp, err := r.client.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	kv := resp.Kvs[0]
	return &model.ConfigItem{
		Key:      string(kv.Key),
		Value:    string(kv.Value),
		Revision: kv.ModRevision,
	}, nil
}

// SaveConfig saves a configuration item to etcd.
func (r *ConfigRepository) SaveConfig(ctx context.Context, key, val string) (int64, error) {
	resp, err := r.client.Put(ctx, key, val)
	if err != nil {
		return 0, err
	}
	return resp.Header.Revision, nil
}

// WatchConfig sets up a watch on a given prefix in etcd.
func (r *ConfigRepository) WatchConfig(ctx context.Context, prefix string) clientv3.WatchChan {
	return r.client.Watch(ctx, prefix, clientv3.WithPrefix())
}

func (r *ConfigRepository) GetWithRevision(ctx context.Context, prefix string) (*clientv3.GetResponse, error) {
	resp, err := r.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (r *ConfigRepository) WatchConfigFrom(ctx context.Context, prefix string, startRev int64) clientv3.WatchChan {
	return r.client.Watch(ctx, prefix, clientv3.WithPrefix(), clientv3.WithRev(startRev))
}
