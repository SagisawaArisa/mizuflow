package api

import (
	"context"
	"io"
	"mizuflow/internal/dto/resp"
	"mizuflow/internal/service"
	v1 "mizuflow/pkg/api/v1"
	"mizuflow/pkg/logger"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type StreamProvider interface {
	GetCompensation(lastRev int64) ([]v1.Message, bool)
	GetAllFeatures(ctx context.Context) ([]v1.FeatureFlag, int64)
}

type StreamHandler struct {
	service StreamProvider
	hub     *service.Hub
}

func NewStreamHandler(service StreamProvider, hub *service.Hub) *StreamHandler {
	return &StreamHandler{
		service: service,
		hub:     hub,
	}
}

func (h *StreamHandler) WatchFeature(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	lastRevStr := c.Query("last_rev")
	env := c.Query("env")
	namespacesStr := c.Query("namespace")

	allowedNamespaces := make(map[string]bool)
	if namespacesStr != "" {
		parts := strings.SplitSeq(namespacesStr, ",")
		for p := range parts {
			if strings.TrimSpace(p) != "" {
				allowedNamespaces[strings.TrimSpace(p)] = true
			}
		}
	}

	if env == "" || len(allowedNamespaces) == 0 {
		logger.Warn("client without identity, refused", zap.String("ip", c.ClientIP()))
		return
	} else {
		logger.Info("client connected",
			zap.String("env", env),
			zap.String("namespaces", namespacesStr),
			zap.String("ip", c.ClientIP()),
		)
	}

	var lastRev int64
	if lastRevStr != "" {
		lastRev, _ = strconv.ParseInt(lastRevStr, 10, 64)
	}

	client := &service.Client{
		Send:       make(chan v1.Message, 128),
		Namespaces: allowedNamespaces,
		Env:        env,
	}

	h.hub.Register <- client
	defer func() {
		h.hub.Unregister <- client
	}()

	messages, ok := h.service.GetCompensation(lastRev)
	maxSentRev := lastRev
	if ok {
		for _, msg := range messages {
			// Filter history
			if msg.Env != env || !allowedNamespaces[msg.Namespace] {
				continue
			}
			c.SSEvent("message", msg)
			maxSentRev = msg.Revision
		}
	} else {
		c.SSEvent("reset", "revision_too_old")
	}

	c.Stream(func(w io.Writer) bool {
		select {
		case msg, ok := <-client.Send:
			if !ok {
				return false
			}

			if msg.Type == "ping" {
				c.SSEvent("ping", "pong")
				return true
			}

			if msg.Env != env || !allowedNamespaces[msg.Namespace] {
				return true
			}

			// filter replicated messages
			if msg.Revision <= maxSentRev {
				return true
			}
			c.SSEvent("message", msg)
			maxSentRev = msg.Revision
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}

func (h *StreamHandler) DashboardWatch(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	operator := service.GetOperator(c.Request.Context())
	logger.Info("dashboard client connected",
		zap.String("operator", operator),
		zap.String("ip", c.ClientIP()),
	)

	clientChan := make(chan v1.Message, 128)

	client := &service.Client{
		Send:       clientChan,
		Namespaces: map[string]bool{"*": true},
		Env:        c.Query("env"),
	}

	h.hub.Register <- client
	defer func() {
		h.hub.Unregister <- client
	}()
	c.Stream(func(w io.Writer) bool {
		select {
		case msg, ok := <-client.Send:
			if !ok {
				return false
			}
			if msg.Type == "ping" {
				c.SSEvent("ping", "pong")
				return true
			}
			c.SSEvent("message", msg)
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}

func (h *StreamHandler) FetchAll(c *gin.Context) {
	env := c.Query("env")
	namespacesStr := c.Query("namespace")

	allowedNamespaces := make(map[string]bool)
	if namespacesStr != "" {
		parts := strings.SplitSeq(namespacesStr, ",")
		for p := range parts {
			if strings.TrimSpace(p) != "" {
				allowedNamespaces[strings.TrimSpace(p)] = true
			}
		}
	}

	features, rev := h.service.GetAllFeatures(c.Request.Context())

	// Filter features based on env and namespace
	var filtered []v1.FeatureFlag
	if env == "" && namespacesStr == "" {
		filtered = features
	} else {
		filtered = make([]v1.FeatureFlag, 0, len(features))
		for _, f := range features {
			if env != "" && f.Env != env {
				continue
			}
			if len(allowedNamespaces) > 0 && !allowedNamespaces[f.Namespace] {
				continue
			}
			filtered = append(filtered, f)
		}
	}

	c.JSON(200, resp.SnapshotResponse{
		Data:     filtered,
		Revision: rev,
	})
}
