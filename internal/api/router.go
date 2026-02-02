package api

import (
	"mizuflow/internal/metrics"
	"mizuflow/internal/middleware"
	"mizuflow/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func RegisterRoutes(featureHandler *FeatureHandler, streamHandler *StreamHandler, authHandler *AuthHandler, sdkRepo repository.SDKRepository, rdb *redis.Client, requestsPerSecond int, env string) *gin.Engine {
	r := gin.New()

	// Determine if we should bypass auth (e.g. for load testing)
	bypassAuth := env == "loadtest"

	// Global Middleware
	r.Use(
		middleware.CorsMiddleware(),
		middleware.RequestID(),
		middleware.GinZapLogger(),
		middleware.GinZapRecovery(),
		middleware.HttpMiddleware(),
		middleware.TraceMiddleware(),
	)
	r.SetTrustedProxies(nil)

	// Public Routes
	r.GET("/health", featureHandler.HealthCheck)
	r.GET("/metrics", gin.WrapH(metrics.Handler()))

	// Auth Routes (Public)
	auth := r.Group("/v1/auth")
	{
		auth.POST("/login", authHandler.Login)
		auth.POST("/refresh", authHandler.Refresh)
	}

	// Auth Routes (Protected)
	authProtected := r.Group("/v1/auth")
	authProtected.Use(middleware.JWTMiddleware(true))
	{
		authProtected.GET("/me", authHandler.GetProfile)
		authProtected.POST("/logout", authHandler.Logout)
	}

	// Stream Routes (Protected by SDK Key)
	stream := r.Group("/v1/stream")
	stream.Use(middleware.SDKAuthMiddleware(sdkRepo, bypassAuth))
	{
		stream.GET("/watch", streamHandler.WatchFeature)
		stream.GET("/snapshot", streamHandler.FetchAll)
	}

	admin := r.Group("/v1/admin")
	admin.Use(middleware.JWTMiddleware(true))
	{
		admin.GET("/stream", streamHandler.DashboardWatch)
	}

	// Protected Routes (Control Plane)
	// Enable Dev-Pass=true for debugging
	protected := r.Group("/v1")
	protected.Use(middleware.JWTMiddleware(true))

	// Rate Limiter for Write Operations
	writeLimiter := middleware.RateLimitMiddleware(rdb, requestsPerSecond)

	{
		protected.POST("/feature", writeLimiter, featureHandler.CreateFeature)
		protected.GET("/features", featureHandler.ListFeatures)
		protected.GET("/feature/:key", featureHandler.GetFeature)
		protected.GET("/feature/:key/audits", featureHandler.GetFeatureAudits)
		protected.POST("/feature/:key/rollback", writeLimiter, featureHandler.RollbackFeature)
	}
	return r
}
