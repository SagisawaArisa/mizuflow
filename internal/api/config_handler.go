package api

import (
	"delta-conf/internal/service"
	"io"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ConfigHandler struct {
	service *service.ConfigService
	hub     *service.Hub
}

func NewConfigHandler(service *service.ConfigService, hub *service.Hub) *ConfigHandler {
	return &ConfigHandler{
		service: service,
		hub:     hub,
	}
}

func (h *ConfigHandler) SubscribeConfig(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	lastRevStr := c.Query("last_rev")
	var lastRev int64
	if lastRevStr != "" {
		lastRev, _ = strconv.ParseInt(lastRevStr, 10, 64)
	}

	clientChan := make(chan service.Message, 64)

	messages, ok := h.service.GetCompensation(lastRev)
	if ok {
		for _, msg := range messages {
			c.SSEvent("message", msg)
		}
	} else {
		c.SSEvent("reset", "revision_too_old")
	}

	h.hub.Register <- clientChan

	defer func() {
		h.hub.Unregister <- clientChan
	}()

	c.Stream(func(w io.Writer) bool {
		select {
		case msg, ok := <-clientChan:
			if !ok {
				return false
			}
			c.SSEvent("message", msg)
			return true
		case <-c.Request.Context().Done():
			return false
		}

	})
}
