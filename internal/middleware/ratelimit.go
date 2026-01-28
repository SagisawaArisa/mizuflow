package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"mizuflow/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// RateLimiterConfig defines configuration for the rate limiter
type RateLimiterConfig struct {
	Limit      int           // Requests per second
	Burst      int           // Burst size
	KeyPrefix  string        // Redis key prefix
	Expiration time.Duration // Expiration for keys
}

// tokenBucketScript implements the Token Bucket algorithm.
// Input: ARGV[1]=rate, ARGV[2]=capacity, ARGV[3]=now, ARGV[4]=requested
// Output: { allowed, remaining, reset_after }
var tokenBucketScript = redis.NewScript(`
local tokens_key = KEYS[1]
local ts_key = KEYS[2]
local rate = tonumber(ARGV[1])
local capacity = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

local fill_time = capacity / rate
local ttl = math.ceil(fill_time * 2)

-- Load state
local last_tokens = tonumber(redis.call("get", tokens_key))
if last_tokens == nil then last_tokens = capacity end

local last_ts = tonumber(redis.call("get", ts_key))
if last_ts == nil then last_ts = now end

-- Refill
local delta = math.max(0, now - last_ts)
local filled_tokens = math.min(capacity, last_tokens + (delta * rate))
local allowed = 0
local remaining = filled_tokens
local reset_after = 0

if filled_tokens >= requested then
    allowed = 1
    filled_tokens = filled_tokens - requested
    remaining = filled_tokens
else
    allowed = 0
    remaining = filled_tokens
    reset_after = (requested - filled_tokens) / rate
end

if allowed == 1 then
    redis.call("set", tokens_key, filled_tokens, "EX", ttl)
    redis.call("set", ts_key, now, "EX", ttl)
end

return { allowed, remaining, reset_after }
`)

// Fallback in-memory limiter
type localLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	localLimiters = &sync.Map{}
	cleanupTicker *time.Ticker
	initOnce      sync.Once
)

func initCleanup() {
	initOnce.Do(func() {
		cleanupTicker = time.NewTicker(10 * time.Minute)
		go func() {
			for range cleanupTicker.C {
				now := time.Now()
				localLimiters.Range(func(key, value any) bool {
					l := value.(*localLimiter)
					if now.Sub(l.lastSeen) > 10*time.Minute {
						localLimiters.Delete(key)
					}
					return true
				})
			}
		}()
	})
}

func getLocalLimiter(ip string, r rate.Limit, b int) *rate.Limiter {
	initCleanup() // Ensure cleanup is running

	val, ok := localLimiters.Load(ip)
	if ok {
		l := val.(*localLimiter)
		l.lastSeen = time.Now()
		return l.limiter
	}

	l := &localLimiter{
		limiter:  rate.NewLimiter(r, b),
		lastSeen: time.Now(),
	}
	localLimiters.Store(ip, l)
	return l.limiter
}

// RateLimitMiddleware enforces rate limiting using Redis with a local fail-open strategy.
func RateLimitMiddleware(rdb *redis.Client, requestsPerSecond int) gin.HandlerFunc {
	if requestsPerSecond <= 0 {
		requestsPerSecond = 5 // Default to 5 RPS if invalid
	}
	burst := requestsPerSecond

	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		keyPrefix := "ratelimit:" + clientIP
		tokensKey := keyPrefix + ":tokens"
		tsKey := keyPrefix + ":ts"

		now := float64(time.Now().UnixMicro()) / 1e6

		// 1. Attempt Redis Rate Limit
		keys := []string{tokensKey, tsKey}
		args := []any{
			float64(requestsPerSecond), // rate
			float64(burst),             // capacity
			now,                        // current timestamp
			1,                          // requested tokens
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		result, err := tokenBucketScript.Run(ctx, rdb, keys, args...).Result()

		// 2. Fail-Open Logic (Fallback to Memory)
		if err != nil {
			logger.Warn("Redis rate limit failed, switching to local fallback",
				zap.Error(err),
				zap.String("ip", clientIP))

			limiter := getLocalLimiter(clientIP, rate.Limit(requestsPerSecond), burst)

			// Set degrading headers always for consistency in fallback mode
			c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", requestsPerSecond))

			if !limiter.Allow() {
				c.Header("X-RateLimit-Remaining", "0")
				c.Header("X-RateLimit-Reset", "1") // Static retry value for fallback
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Too Many Requests"})
				return
			}

			c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", int(limiter.Tokens())))
			c.Next()
			return
		}

		// 3. Process Redis Result
		resSlice, ok := result.([]any)
		if !ok || len(resSlice) != 3 {
			logger.Error("Invalid Redis rate limit response", zap.Any("response", result))
			c.Next() // Fail open on protocol error
			return
		}

		allowed := helperInt(resSlice[0]) == 1
		remaining := helperFloat(resSlice[1])
		resetAfter := helperFloat(resSlice[2])

		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", requestsPerSecond))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", int(remaining)))

		resetTime := time.Now().Add(time.Duration(resetAfter * float64(time.Second)))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", resetTime.Unix()))

		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Too Many Requests"})
			return
		}

		c.Next()
	}
}

func helperInt(v any) int64 {
	if val, ok := v.(int64); ok {
		return val
	}
	if val, ok := v.(float64); ok {
		return int64(val)
	}
	return 0
}

func helperFloat(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int64:
		return float64(val)
	default:
		return 0
	}
}
