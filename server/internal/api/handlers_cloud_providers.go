package api

import (
	"context"
	"crypto/subtle"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pangjian-pj/cotriage-dev/server/internal/model"
	"github.com/pangjian-pj/cotriage-dev/server/internal/store"
)

func cloudID(c *gin.Context) (int64, bool) {
	id, e := strconv.ParseInt(c.Param("id"), 10, 64)
	if e != nil || id <= 0 {
		c.JSON(400, gin.H{"error": "invalid id"})
		return 0, false
	}
	return id, true
}
func cloudError(c *gin.Context, e error) {
	switch e {
	case store.ErrNotFound:
		c.JSON(404, gin.H{"error": "not found"})
	case store.ErrBadRequest:
		c.JSON(400, gin.H{"error": "invalid provider"})
	case store.ErrConflict:
		c.JSON(409, gin.H{"error": "provider state conflict"})
	default:
		c.JSON(500, gin.H{"error": e.Error()})
	}
}
func (h *Handler) CreateCloudProvider(c *gin.Context) {
	var r model.CreateCloudProviderRequest
	if e := c.ShouldBindJSON(&r); e != nil {
		c.JSON(400, gin.H{"error": e.Error()})
		return
	}
	p, e := h.CloudProvidersStore.Create(c, r)
	if e != nil {
		cloudError(c, e)
		return
	}
	c.JSON(201, p)
}
func (h *Handler) ListCloudProviders(c *gin.Context) {
	p, e := h.CloudProvidersStore.List(c)
	if e != nil {
		cloudError(c, e)
		return
	}
	c.JSON(200, gin.H{"items": p, "page": 1, "page_size": len(p), "total": len(p)})
}
func (h *Handler) GetCloudProvider(c *gin.Context) {
	id, ok := cloudID(c)
	if !ok {
		return
	}
	p, e := h.CloudProvidersStore.Get(c.Request.Context(), id)
	if e != nil {
		cloudError(c, e)
		return
	}
	c.JSON(http.StatusOK, p)
}
func (h *Handler) PatchCloudProvider(c *gin.Context) {
	id, ok := cloudID(c)
	if !ok {
		return
	}
	var r model.PatchCloudProviderRequest
	if e := c.ShouldBindJSON(&r); e != nil {
		c.JSON(400, gin.H{"error": e.Error()})
		return
	}
	p, e := h.CloudProvidersStore.Patch(c, id, r)
	if e != nil {
		cloudError(c, e)
		return
	}
	c.JSON(200, p)
}
func (h *Handler) DeleteCloudProvider(c *gin.Context) {
	id, ok := cloudID(c)
	if !ok {
		return
	}
	if e := h.CloudProvidersStore.Delete(c, id); e != nil {
		cloudError(c, e)
		return
	}
	c.Status(204)
}
func (h *Handler) ActivateCloudProvider(c *gin.Context) {
	id, ok := cloudID(c)
	if !ok {
		return
	}
	p, e := h.CloudProvidersStore.Activate(c, id)
	if e != nil {
		cloudError(c, e)
		return
	}
	c.Header("ETag", p.Etag)
	c.JSON(200, p)
}
func (h *Handler) GetCloudConfig(c *gin.Context) {
	if token := strings.TrimSpace(os.Getenv("CLOUD_GATEWAY_CONFIG_TOKEN")); token != "" {
		provided := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
		if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "cloud gateway authorization required"})
			return
		}
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	p, e := h.CloudProvidersStore.Active(ctx)
	if e != nil {
		cloudError(c, e)
		return
	}
	now := time.Now().UTC()
	if p == nil {
		c.JSON(http.StatusOK, model.CloudConfigResponse{Enabled: false, Reason: "NO_ACTIVE_CLOUD_PROVIDER", GeneratedAt: now})
		return
	}
	if c.GetHeader("If-None-Match") == p.Etag {
		c.Header("ETag", p.Etag)
		c.Status(304)
		return
	}
	c.Header("ETag", p.Etag)
	c.Header("Cache-Control", "no-store")
	c.JSON(200, model.CloudConfigResponse{Enabled: true, ConfigVersion: p.ConfigVersion, Etag: p.Etag, Provider: p, Prompt: model.CloudPromptConfig{Version: "cloud-diag-prompt-v1", RawLogMaxChars: 2000, ContextWindow: "5m", ContextMaxEvents: 20, ContextMaxTokens: 3000}, Reward: model.CloudRewardConfig{LatencySLAMS: 5000, TokenBudgetPerRequest: 2000}, GeneratedAt: now})
}
