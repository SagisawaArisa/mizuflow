package middleware

import (
	"mizuflow/pkg/logger"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func init() {
	logger.InitLogger("test")
}

func TestRateLimitMiddleware_RedisFailure_FailsOpen(t *testing.T) {
	// Setup Redis client with unreachable address to force connection failure
	rdb := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:0", // Invalid port
		DialTimeout: 10 * time.Millisecond,
		ReadTimeout: 10 * time.Millisecond,
		MaxRetries:  0,
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RateLimitMiddleware(rdb, 10))
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)

	r.ServeHTTP(w, req)

	// Should fail open (Status 200) despite Redis being down
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 (Fail Open), got %d", w.Code)
	}

	// Verify Fallback logic utilized local map by checking Header
	if val := w.Header().Get("X-RateLimit-Limit"); val != "10" {
		t.Errorf("Expected X-RateLimit-Limit header '10', got '%s'", val)
	}
}
