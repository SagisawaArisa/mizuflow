package middleware

import (
	"mizuflow/internal/repository"

	"github.com/gin-gonic/gin"
)

func SDKAuthMiddleware(repo repository.SDKRepository, bypassAuth bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if bypassAuth {
			c.Next()
			return
		}

		apiKey := c.GetHeader("X-Mizu-Key")
		env := c.Query("env")

		if apiKey == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "missing API key"})
			return
		}

		ok, err := repo.ValidateAPIKey(c.Request.Context(), apiKey, env)
		if err != nil || !ok {
			c.AbortWithStatusJSON(403, gin.H{"error": "forbidden"})
			return
		}

		c.Next()
	}
}
