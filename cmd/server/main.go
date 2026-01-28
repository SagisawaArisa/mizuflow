package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mizuflow/internal/api"
	"mizuflow/internal/config"
	"mizuflow/internal/metrics"
	"mizuflow/internal/model"
	"mizuflow/internal/repository"
	"mizuflow/internal/service"
	"mizuflow/pkg/logger"

	"github.com/redis/go-redis/v9"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	// 1. Load Configuration
	cfg := config.Load()

	// Initialize logger
	logger.InitLogger(cfg.Server.Environment)
	defer logger.Sync()

	if err := run(cfg); err != nil {
		logger.Error("application startup failed", zap.Error(err))
		os.Exit(1)
	}
}

func run(cfg *config.Config) error {
	// 2. Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 3. Initialize Infrastructure
	rdb, err := initRedis(cfg.Redis)
	if err != nil {
		return err
	}
	defer rdb.Close()

	etcdCli, err := initEtcd(cfg.Etcd)
	if err != nil {
		return err
	}
	defer etcdCli.Close()

	db, err := initDB(cfg.MySQL)
	if err != nil {
		return err
	}

	// 4. Initialize Repositories
	etcdRepo := repository.NewFeatureRepository(etcdCli)
	mysqlRepo := repository.NewAuditRepository(db)
	featureRepo := repository.NewFeatureMasterRepository(db)
	outboxRepo := repository.NewOutboxRepository(db)
	sdkRepo := repository.NewSDKKeyRepository(db)

	// 5. Initialize Services
	observer := metrics.NewPrometheusObserver()
	hub := service.NewHub(observer, cfg.Stream.HeartbeatInterval)

	svc := service.NewFeatureService(db, etcdRepo, mysqlRepo, featureRepo, outboxRepo, hub)
	authSvc := service.NewAuthService(rdb, cfg.Auth.AccessTokenTTL, cfg.Auth.RefreshTokenTTL)

	// 6. Initialize & Start Workers (Background Tasks)
	outboxWorker := service.NewOutboxWorker(outboxRepo, etcdRepo, cfg.Workers.OutboxInterval)
	reconciler := service.NewReconciler(etcdCli, etcdRepo, featureRepo, service.ReconcilerConfig{
		Interval:   cfg.Workers.ReconcilerInterval,
		BatchSize:  cfg.Workers.ReconcilerBatchSize,
		BatchDelay: cfg.Workers.ReconcilerBatchDelay,
	})

	// Start background routines
	go func() {
		logger.Info("starting outbox worker")
		outboxWorker.Run(ctx)
	}()
	go func() {
		logger.Info("starting reconciler")
		reconciler.Run(ctx)
	}()
	go func() {
		logger.Info("starting hub")
		hub.Run()
	}()
	go func() {
		logger.Info("starting feature service watcher")
		svc.Run(ctx)
	}()

	// 7. Setup HTTP Server
	r := api.RegisterRoutes(
		api.NewFeatureHandler(svc, hub),
		api.NewStreamHandler(svc, hub),
		api.NewAuthHandler(authSvc),
		sdkRepo,
	)

	srv := &http.Server{
		Addr:    cfg.Server.Port,
		Handler: r,
	}

	// 8. Start Server
	go func() {
		logger.Info("server starting",
			zap.String("addr", cfg.Server.Port),
			zap.String("env", cfg.Server.Environment))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server listen failed", zap.Error(err))
		}
	}()

	// 9. Graceful Shutdown Signal Wait
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down server...")

	// Create a deadline to wait for current requests to complete
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Signal all workers to stop
	cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	logger.Info("server exited properly")
	return nil
}

// -- Infrastructure Initializers --

func initRedis(cfg config.RedisConfig) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}
	return rdb, nil
}

func initEtcd(cfg config.EtcdConfig) (*clientv3.Client, error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: cfg.DialTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to etcd: %w", err)
	}
	return client, nil
}

func initDB(cfg config.MySQLConfig) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mysql: %w", err)
	}

	// Simple auto-migrate for dev convenience
	// In production, you might want to use a proper migration tool like golang-migrate
	err = db.AutoMigrate(
		&model.FeatureMaster{},
		&model.FeatureAudit{},
		&model.OutboxTask{},
		&model.SDKClient{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return db, nil
}
