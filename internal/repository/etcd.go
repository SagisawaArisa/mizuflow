package repository

import (
	"context"
	"encoding/json"
	"errors"
	v1 "mizuflow/pkg/api/v1"

	clientv3 "go.etcd.io/etcd/client/v3"
)

var ErrFeatureNotFound = errors.New("feature not found")

type EtcdInterface interface {
	clientv3.KV
	clientv3.Watcher
	Close() error
}

type FeatureRepository struct {
	client EtcdInterface
}

func NewFeatureRepository(client EtcdInterface) *FeatureRepository {
	return &FeatureRepository{
		client: client,
	}
}

// GetFeature retrieves a feature item by key from etcd.
func (r *FeatureRepository) GetFeature(ctx context.Context, key string) (*v1.FeatureFlag, error) {
	resp, err := r.client.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, ErrFeatureNotFound
	}
	kv := resp.Kvs[0]
	return &v1.FeatureFlag{
		Key:      string(kv.Key),
		Value:    string(kv.Value),
		Revision: kv.ModRevision,
	}, nil
}

// SaveFeature saves a feature item to etcd.
func (r *FeatureRepository) SaveFeature(ctx context.Context, key, val string) (int64, error) {
	resp, err := r.client.Put(ctx, key, val)
	if err != nil {
		return 0, err
	}
	return resp.Header.Revision, nil
}

// SaveFeatureIfNewer saves a feature item to etcd only if the new version is greater than the existing one.(CAS)
func (r *FeatureRepository) SaveFeatureIfNewer(ctx context.Context, key string, newValue v1.FeatureFlag) (int64, error) {
	const maxRetries = 3
	var retries int

	for {
		resp, err := r.client.Get(ctx, key)
		if err != nil {
			return 0, err
		}

		// Calculate serialized value once
		val := newValue.ToJSON()

		if len(resp.Kvs) == 0 {
			txn := r.client.Txn(ctx).
				If(clientv3.Compare(clientv3.CreateRevision(key), "=", 0)).
				Then(clientv3.OpPut(key, val))

			tResp, err := txn.Commit()
			if err != nil {
				return 0, err
			}
			if !tResp.Succeeded {
				// Contention detected
				retries++
				if retries > maxRetries {
					return 0, errors.New("max retries exceeded for SaveFeatureIfNewer")
				}
				continue
			}
			return tResp.Header.Revision, nil
		}

		// Key exists, parse the value to check stored version
		var currentFlag v1.FeatureFlag
		kv := resp.Kvs[0]
		if err := json.Unmarshal(kv.Value, &currentFlag); err != nil {
			return 0, err
		} else {
			// If stored Logic Version >= new Logic Version, Do Nothing (Idempotency).
			if currentFlag.Version >= newValue.Version {
				return kv.ModRevision, nil
			}
		}

		// CAS Update: Ensure we are updating the exact Etcd Revision we just read
		txn := r.client.Txn(ctx).
			If(clientv3.Compare(clientv3.ModRevision(key), "=", kv.ModRevision)).
			Then(clientv3.OpPut(key, val))

		tResp, err := txn.Commit()
		if err != nil {
			return 0, err
		}

		if tResp.Succeeded {
			return tResp.Header.Revision, nil
		}
		// If CAS failed (someone else updated in between), loop again
		retries++
		if retries > maxRetries {
			return 0, errors.New("max retries exceeded for SaveFeatureIfNewer")
		}
	}
}

// WatchFeature sets up a watch on a given prefix in etcd.
func (r *FeatureRepository) WatchFeature(ctx context.Context, prefix string) clientv3.WatchChan {
	return r.client.Watch(ctx, prefix, clientv3.WithPrefix())
}

func (r *FeatureRepository) GetWithRevision(ctx context.Context, prefix string) (*clientv3.GetResponse, error) {
	resp, err := r.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (r *FeatureRepository) WatchFeatureFrom(ctx context.Context, prefix string, startRev int64) clientv3.WatchChan {
	return r.client.Watch(ctx, prefix, clientv3.WithPrefix(), clientv3.WithRev(startRev))
}

func (r *FeatureRepository) Health(ctx context.Context) error {
	_, err := r.client.Get(ctx, "health_check")
	return err
}
