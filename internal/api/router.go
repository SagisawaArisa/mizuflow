package api

import (
	"mizuflow/internal/metrics"
	"mizuflow/internal/middleware"
	"mizuflow/internal/repository"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(featureHandler *FeatureHandler, streamHandler *StreamHandler, authHandler *AuthHandler, sdkRepo repository.SDKRepository) *gin.Engine {
	r := gin.New()

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
	stream.Use(middleware.SDKAuthMiddleware(sdkRepo))
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
	{
		protected.POST("/feature", featureHandler.CreateFeature)
		protected.GET("/features", featureHandler.ListFeatures)
		protected.GET("/feature/:key", featureHandler.GetFeature)
		protected.GET("/feature/:key/audits", featureHandler.GetFeatureAudits)
		protected.POST("/feature/:key/rollback", featureHandler.RollbackFeature)
	}
	return r
}
