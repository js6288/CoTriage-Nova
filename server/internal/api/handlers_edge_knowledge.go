package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pangjian-pj/cotriage-dev/server/internal/model"
	"github.com/pangjian-pj/cotriage-dev/server/internal/store"
)

func (h *Handler) CreateEdgeRule(c *gin.Context) {
	if !h.requireEdgeKnowledgeStore(c) {
		return
	}
	var req model.UpsertEdgeRuleRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()
	item, err := h.EdgeKnowledgeStore.CreateRule(ctx, req)
	writeKnowledgeResult(c, item, err, http.StatusCreated)
}

func (h *Handler) PatchEdgeRule(c *gin.Context) {
	if !h.requireEdgeKnowledgeStore(c) {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req model.UpsertEdgeRuleRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()
	item, err := h.EdgeKnowledgeStore.PatchRule(ctx, id, req)
	writeKnowledgeResult(c, item, err, http.StatusOK)
}

func (h *Handler) ListEdgeRules(c *gin.Context) {
	if !h.requireEdgeKnowledgeStore(c) {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()
	q := listKnowledgeQuery(c)
	items, total, err := h.EdgeKnowledgeStore.ListRules(ctx, q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	page, ps := normalizeResponsePage(q.Page, q.PageSize)
	c.JSON(http.StatusOK, model.ListKnowledgeResponse[model.EdgeRule]{
		Items:    items,
		Page:     page,
		PageSize: ps,
		Total:    total,
	})
}

func (h *Handler) DeleteEdgeRule(c *gin.Context) {
	if !h.requireEdgeKnowledgeStore(c) {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()
	err := h.EdgeKnowledgeStore.DeleteRule(ctx, id)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) CreateEdgeTemplate(c *gin.Context) {
	if !h.requireEdgeKnowledgeStore(c) {
		return
	}
	var req model.UpsertEdgeTemplateRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()
	item, err := h.EdgeKnowledgeStore.CreateTemplate(ctx, req)
	writeKnowledgeResult(c, item, err, http.StatusCreated)
}

func (h *Handler) PatchEdgeTemplate(c *gin.Context) {
	if !h.requireEdgeKnowledgeStore(c) {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req model.UpsertEdgeTemplateRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()
	item, err := h.EdgeKnowledgeStore.PatchTemplate(ctx, id, req)
	writeKnowledgeResult(c, item, err, http.StatusOK)
}

func (h *Handler) ListEdgeTemplates(c *gin.Context) {
	if !h.requireEdgeKnowledgeStore(c) {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()
	q := listKnowledgeQuery(c)
	items, total, err := h.EdgeKnowledgeStore.ListTemplates(ctx, q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	page, ps := normalizeResponsePage(q.Page, q.PageSize)
	c.JSON(http.StatusOK, model.ListKnowledgeResponse[model.EdgeTemplate]{
		Items:    items,
		Page:     page,
		PageSize: ps,
		Total:    total,
	})
}

func (h *Handler) CreateEdgeTemplateFromCluster(c *gin.Context) {
	if !h.requireEdgeKnowledgeStore(c) {
		return
	}
	var req model.CreateTemplateFromClusterRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()
	item, created, err := h.EdgeKnowledgeStore.CreateTemplateFromCluster(ctx, req)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	code := http.StatusOK
	if created {
		code = http.StatusCreated
	}
	c.JSON(code, gin.H{"created": created, "item": item})
}

func (h *Handler) DeleteEdgeTemplate(c *gin.Context) {
	if !h.requireEdgeKnowledgeStore(c) {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()
	err := h.EdgeKnowledgeStore.DeleteTemplate(ctx, id)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) CreateEdgeRuleSet(c *gin.Context) {
	if !h.requireEdgeKnowledgeStore(c) {
		return
	}
	var req model.CreateSetRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	item, err := h.EdgeKnowledgeStore.CreateRuleSet(ctx, req)
	writeKnowledgeResult(c, item, err, http.StatusCreated)
}

func (h *Handler) ActivateEdgeRuleSet(c *gin.Context) {
	if !h.requireEdgeKnowledgeStore(c) {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	cfg, err := h.EdgeKnowledgeStore.ActivateRuleSet(ctx, id)
	writeKnowledgeResult(c, cfg, err, http.StatusOK)
}

func (h *Handler) CreateEdgeTemplateSet(c *gin.Context) {
	if !h.requireEdgeKnowledgeStore(c) {
		return
	}
	var req model.CreateSetRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	item, err := h.EdgeKnowledgeStore.CreateTemplateSet(ctx, req)
	writeKnowledgeResult(c, item, err, http.StatusCreated)
}

func (h *Handler) ActivateEdgeTemplateSet(c *gin.Context) {
	if !h.requireEdgeKnowledgeStore(c) {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	cfg, err := h.EdgeKnowledgeStore.ActivateTemplateSet(ctx, id)
	writeKnowledgeResult(c, cfg, err, http.StatusOK)
}

func (h *Handler) requireEdgeKnowledgeStore(c *gin.Context) bool {
	if h.EdgeKnowledgeStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "edge knowledge store is not configured"})
		return false
	}
	return true
}

func bindJSON(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return false
	}
	return true
}

func parseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, false
	}
	return id, true
}

func listKnowledgeQuery(c *gin.Context) store.ListKnowledgeQuery {
	page, _ := strconv.Atoi(strings.TrimSpace(c.Query("page")))
	pageSize, _ := strconv.Atoi(strings.TrimSpace(c.Query("page_size")))
	return store.ListKnowledgeQuery{
		Status:   strings.TrimSpace(c.Query("status")),
		EdgePred: strings.TrimSpace(c.Query("edge_pred")),
		Q:        strings.TrimSpace(c.Query("q")),
		Page:     page,
		PageSize: pageSize,
	}
}

func normalizeResponsePage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	return page, pageSize
}

func writeKnowledgeResult(c *gin.Context, payload any, err error, code int) {
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.JSON(code, payload)
}

func writeKnowledgeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	case errors.Is(err, store.ErrBadRequest):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
