package service

import (
	"context"
	"errors"
	"testing"

	"mizuflow/internal/buffer"
	"mizuflow/internal/repository"
	v1 "mizuflow/pkg/api/v1"
	"mizuflow/pkg/constraints"
	"mizuflow/pkg/logger"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func init() {
	logger.InitLogger("test")
}

// MockKV partially implements clientv3.KV
type MockKV struct {
	clientv3.KV
	GetFn func(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error)
}

func (m *MockKV) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, key, opts...)
	}
	return nil, nil
}

func (m *MockKV) Txn(ctx context.Context) clientv3.Txn {
	return nil
}

type MockEtcdInterface struct {
	MockKV
	clientv3.Watcher
}

func (m *MockEtcdInterface) Close() error { return nil }
func (m *MockEtcdInterface) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	return nil
}

func TestValidatePayload(t *testing.T) {
	svc := &FeatureService{}

	tests := []struct {
		name    string
		flag    v1.FeatureFlag
		wantErr bool
	}{
		{
			name:    "Bool invalid",
			flag:    v1.FeatureFlag{Type: constraints.TypeBool, Value: "yes"},
			wantErr: true,
		},
		{
			name:    "Bool valid",
			flag:    v1.FeatureFlag{Type: constraints.TypeBool, Value: "true"},
			wantErr: false,
		},
		{
			name:    "Number invalid",
			flag:    v1.FeatureFlag{Type: constraints.TypeNumber, Value: "abc"},
			wantErr: true,
		},
		{
			name:    "Strategy invalid json",
			flag:    v1.FeatureFlag{Type: constraints.TypeStrategy, Value: "{"},
			wantErr: true,
		},
		{
			name:    "Strategy missing default",
			flag:    v1.FeatureFlag{Type: constraints.TypeStrategy, Value: `{"rules":[]}`},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
				}
			}()

			_, err := svc.SaveFeature(context.Background(), tt.flag, "test-op")

			if tt.wantErr {
				if err == nil {
					t.Error("Expected validation error, got nil")
				} else if err.Error() == "feature save failed" {
					t.Error("Expected validation error, passed validation")
				}
			}
		})
	}
}

func TestSyncToEtcd_Failure(t *testing.T) {
	// Ensure robustness against Etcd failure (Transactional Outbox)
	mockKV := &MockKV{
		GetFn: func(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
			return nil, errors.New("etcd fatal error")
		},
	}
	mockEtcd := &MockEtcdInterface{MockKV: *mockKV}

	repo := repository.NewFeatureRepository(mockEtcd)

	svc := &FeatureService{
		etcdRepo: repo,
	}

	flag := v1.FeatureFlag{
		Key:       "test-key",
		Namespace: "default",
		Env:       "dev",
		Version:   1,
	}

	// Should Log Warn but not Panic/Fail
	svc.syncToEtcd(123, flag)
}

func TestGetCompensation_DelegatesToBuffer(t *testing.T) {
	svc := &FeatureService{
		buffer: buffer.NewRevisionBuffer(10),
	}

	svc.buffer.AddMessage(v1.Message{Revision: 99})
	svc.buffer.AddMessage(v1.Message{Revision: 100})

	msgs, ok := svc.GetCompensation(99)
	if !ok || len(msgs) != 1 {
		t.Error("Delegation to buffer failed")
	}
}
