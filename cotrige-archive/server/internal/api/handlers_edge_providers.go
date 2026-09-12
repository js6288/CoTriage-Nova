package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pangjian-pj/cotriage-dev/server/internal/model"
	"github.com/pangjian-pj/cotriage-dev/server/internal/store"
)

func (h *Handler) CreateEdgeProvider(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	var req model.CreateEdgeProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	p, err := h.EdgeProvidersStore.Create(ctx, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, p)
}

func (h *Handler) PatchEdgeProvider(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req model.PatchEdgeProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	p, err := h.EdgeProvidersStore.Patch(ctx, id, req)
	if err != nil {
		switch err {
		case store.ErrNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		case store.ErrBadRequest:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid state"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *Handler) GetEdgeProvider(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 6*time.Second)
	defer cancel()

	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	p, err := h.EdgeProvidersStore.Get(ctx, id)
	if err != nil {
		switch err {
		case store.ErrNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *Handler) ListEdgeProviders(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	page, _ := strconv.Atoi(strings.TrimSpace(c.Query("page")))
	pageSize, _ := strconv.Atoi(strings.TrimSpace(c.Query("page_size")))

	q := store.ListEdgeProvidersQuery{
		Page:     page,
		PageSize: pageSize,
	}

	if s := strings.TrimSpace(c.Query("status")); s != "" {
		st := model.EdgeProviderStatus(s)
		q.Status = &st
	}
	if a := strings.TrimSpace(c.Query("active")); a != "" {
		v := a == "true" || a == "1"
		q.Active = &v
	}
	q.Q = strings.TrimSpace(c.Query("q"))

	items, total, err := h.EdgeProvidersStore.List(ctx, q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	c.JSON(http.StatusOK, model.ListEdgeProvidersResponse{Items: items, Page: q.Page, PageSize: q.PageSize, Total: total})
}

func (h *Handler) DeleteEdgeProvider(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	err = h.EdgeProvidersStore.SoftDelete(ctx, id)
	if err != nil {
		switch err {
		case store.ErrNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		case store.ErrConflict:
			c.JSON(http.StatusConflict, gin.H{"error": "active provider cannot be deleted"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ActivateEdgeProvider(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	cfg, err := h.EdgeProvidersStore.Activate(ctx, id)
	if err != nil {
		switch err {
		case store.ErrNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		case store.ErrBadRequest:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid state"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.Header("ETag", cfg.Etag)
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, cfg)
}

func (h *Handler) GetEdgeConfig(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 6*time.Second)
	defer cancel()

	cfg, err := h.EdgeProvidersStore.GetActiveConfig(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if h.EdgeKnowledgeStore != nil {
		rules, ruleErr := h.EdgeKnowledgeStore.ActiveRules(ctx)
		if ruleErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": ruleErr.Error()})
			return
		}
		templates, templateErr := h.EdgeKnowledgeStore.ActiveTemplates(ctx)
		if templateErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": templateErr.Error()})
			return
		}
		cfg.Rules = rules
		cfg.Templates = templates
		cfg.Etag = combineEdgeConfigETag(cfg.Etag, rules.Etag, templates.Etag)
		if rules.Version > cfg.ConfigVersion {
			cfg.ConfigVersion = rules.Version
		}
		if templates.Version > cfg.ConfigVersion {
			cfg.ConfigVersion = templates.Version
		}
	}
	if h.ClassifierStore != nil {
		classifier, classifierErr := h.ClassifierStore.Active(ctx)
		if classifierErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": classifierErr.Error()})
			return
		}
		cfg.Classifier = classifier
		classifierEtag := ""
		if classifier != nil {
			classifierEtag = classifier.Etag
			if classifier.ConfigVersion > cfg.ConfigVersion {
				cfg.ConfigVersion = classifier.ConfigVersion
			}
		}
		cfg.Etag = combineEdgeConfigETag(cfg.Etag, classifierEtag)
	}

	// Conditional request.
	if inm := strings.TrimSpace(c.GetHeader("If-None-Match")); inm != "" && inm == cfg.Etag && cfg.Etag != "" {
		c.Header("ETag", cfg.Etag)
		c.Header("Cache-Control", "no-store")
		c.Status(http.StatusNotModified)
		return
	}

	if cfg.Etag != "" {
		c.Header("ETag", cfg.Etag)
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, cfg)
}

func combineEdgeConfigETag(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		h.Write([]byte("|"))
		h.Write([]byte(part))
	}
	return `W/"edge-config-` + hex.EncodeToString(h.Sum(nil))[:16] + `"`
}
