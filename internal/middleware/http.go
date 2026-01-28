package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

func HttpMiddleware() gin.HandlerFunc {
	summary := promauto.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "mizuflow_http_duration_seconds",
			Help: "Duration of HTTP requests.",
		},
		[]string{"path", "method", "status"},
	)
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		summary.WithLabelValues(c.FullPath(), c.Request.Method, status).Observe(duration)
	}
}
