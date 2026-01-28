package middleware

import (
	"mizuflow/internal/service"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func JWTMiddleware(devMode bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if devMode && c.GetHeader("X-Dev-Pass") == "true" {
			// Inject Mock Admin
			ctx := service.WithOperator(c.Request.Context(), &service.OperatorInfo{
				UserID: "9999",
				Name:   "dev-admin",
				Role:   "admin",
			})
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			return
		}

		tokenString := ""
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				tokenString = parts[1]
			}
		}

		if tokenString == "" {
			tokenString = c.Query("token")
		}

		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header missing"})
			return
		}

		token, err := jwt.ParseWithClaims(tokenString, &service.UserClaims{}, func(t *jwt.Token) (interface{}, error) {
			return service.SignedKey, nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid access token"})
			return
		}

		claims, ok := token.Claims.(*service.UserClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			return
		}

		op := &service.OperatorInfo{
			UserID: claims.UserID,
			Name:   claims.Username,
			Role:   claims.Role,
		}

		ctx := service.WithOperator(c.Request.Context(), op)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
