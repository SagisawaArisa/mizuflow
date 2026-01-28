package api

import (
	"context"
	"mizuflow/internal/dto/req"
	"mizuflow/internal/dto/resp"
	"mizuflow/internal/service"
	v1 "mizuflow/pkg/api/v1"

	"github.com/gin-gonic/gin"
)

type FeatureProvider interface {
	SaveFeature(ctx context.Context, flag v1.FeatureFlag, operator string) (int, error)
	GetFeature(ctx context.Context, namespace, env, key string) (*resp.FeatureItem, error)
	ListFeatures(ctx context.Context, namespace, env, search string) ([]resp.FeatureItem, error)
	GetFeatureAudits(ctx context.Context, namespace, env, key string) ([]resp.AuditLogItem, error)
	RollbackFeature(ctx context.Context, namespace, env, key string, auditID uint, operator string) (int, error)
	Health(ctx context.Context) error
}

type FeatureHandler struct {
	service FeatureProvider
	hub     *service.Hub
}

func NewFeatureHandler(service FeatureProvider, hub *service.Hub) *FeatureHandler {
	return &FeatureHandler{
		service: service,
		hub:     hub,
	}
}

func (h *FeatureHandler) CreateFeature(c *gin.Context) {
	var r req.CreateFeatureRequest

	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(400, gin.H{"error": "JSON format error"})
		return
	}
	operator := service.GetOperator(c.Request.Context())
	rev, err := h.service.SaveFeature(c.Request.Context(), v1.FeatureFlag{
		Namespace: r.Namespace,
		Env:       r.Env,
		Key:       r.Key,
		Value:     r.Value,
		Version:   0,
		Type:      r.Type,
	}, operator)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, resp.CreateFeatureResponse{Version: rev})
}

func (h *FeatureHandler) GetFeature(c *gin.Context) {
	var r req.GetFeatureRequest
	if err := c.ShouldBindUri(&r); err != nil {
		c.JSON(400, gin.H{"error": "invalid key"})
		return
	}
	if err := c.ShouldBindQuery(&r); err != nil {
		c.JSON(400, gin.H{"error": "invalid params"})
		return
	}

	featureItem, err := h.service.GetFeature(c.Request.Context(), r.Namespace, r.Env, r.Key)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, resp.GetFeatureResponse{FeatureItem: featureItem})
}

func (h *FeatureHandler) ListFeatures(c *gin.Context) {
	namespace := c.Query("namespace")
	env := c.Query("env")
	search := c.Query("search")

	features, err := h.service.ListFeatures(c.Request.Context(), namespace, env, search)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, features)
}

func (h *FeatureHandler) GetFeatureAudits(c *gin.Context) {
	key := c.Param("key")
	namespace := c.DefaultQuery("namespace", "default")
	env := c.DefaultQuery("env", "dev")

	audits, err := h.service.GetFeatureAudits(c.Request.Context(), namespace, env, key)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, audits)
}

func (h *FeatureHandler) RollbackFeature(c *gin.Context) {
	key := c.Param("key")
	var r req.RollbackFeatureRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(400, gin.H{"error": "invalid request body"})
		return
	}
	operator := service.GetOperator(c.Request.Context())
	rev, err := h.service.RollbackFeature(c.Request.Context(), r.Namespace, r.Env, key, uint(r.AuditID), operator)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, resp.RollbackFeatureResponse{Version: rev})
}

func (h *FeatureHandler) HealthCheck(c *gin.Context) {
	if err := h.service.Health(c.Request.Context()); err != nil {
		c.JSON(503, gin.H{"status": "unhealthy", "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}
