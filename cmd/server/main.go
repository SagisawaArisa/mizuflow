package main

import (
	"context"
	"delta-conf/internal/api"
	"delta-conf/internal/repository"
	"delta-conf/pkg/logger"

	"delta-conf/internal/service"

	"github.com/gin-gonic/gin"
)

func main() {
	// initialize logger
	logger.InitLogger("dev")
	defer logger.Sync()

	// initialize etcd repository and config service
	repo, err := repository.NewConfigRepository([]string{"localhost:2379"})
	if err != nil {
		panic(err)
	}
	hub := service.NewHub()
	svc := service.NewConfigService(repo, hub)
	handler := api.NewConfigHandler(svc, hub)
	go hub.Run()
	go svc.StartWatch(context.Background())

	r := gin.New()
	r.Use(
		logger.RequestID(),
		logger.GinZapLogger(),
		logger.GinZapRecovery(),
	)
	// If you don't need trusted proxies, you can set it to nil
	r.SetTrustedProxies(nil)

	r.GET("/config/subscribe", handler.SubscribeConfig)

	r.POST("/config", func(c *gin.Context) {
		var req struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "JSON 格式错误"})
			return
		}
		rev, err := repo.SaveConfig(c.Request.Context(), req.Key, req.Value)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"revision": rev})
	})
	r.GET("/config/:key", func(c *gin.Context) {
		key := c.Param("key")
		configItem, err := repo.GetConfig(c.Request.Context(), key)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, configItem)
	})
	r.Run(":8080")
}
