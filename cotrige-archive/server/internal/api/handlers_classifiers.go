package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pangjian-pj/cotriage-dev/server/internal/model"
	"github.com/pangjian-pj/cotriage-dev/server/internal/store"
)

func (h *Handler) CreateClassifierConfig(c *gin.Context) {
	if !h.requireClassifierStore(c) {
		return
	}
	var req model.UpsertClassifierConfigRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()
	item, err := h.ClassifierStore.Create(ctx, req)
	writeKnowledgeResult(c, item, err, http.StatusCreated)
}

func (h *Handler) PatchClassifierConfig(c *gin.Context) {
	if !h.requireClassifierStore(c) {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req model.UpsertClassifierConfigRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()
	item, err := h.ClassifierStore.Patch(ctx, id, req)
	writeKnowledgeResult(c, item, err, http.StatusOK)
}

func (h *Handler) ListClassifierConfigs(c *gin.Context) {
	if !h.requireClassifierStore(c) {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()
	q := listKnowledgeQuery(c)
	items, total, err := h.ClassifierStore.List(ctx, q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	page, pageSize := normalizeResponsePage(q.Page, q.PageSize)
	c.JSON(http.StatusOK, model.ListClassifierConfigsResponse{Items: items, Page: page, PageSize: pageSize, Total: total})
}

func (h *Handler) DeleteClassifierConfig(c *gin.Context) {
	if !h.requireClassifierStore(c) {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()
	err := h.ClassifierStore.Delete(ctx, id)
	if err != nil {
		writeClassifierError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ActivateClassifierConfig(c *gin.Context) {
	if !h.requireClassifierStore(c) {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()
	item, err := h.ClassifierStore.Activate(ctx, id)
	writeKnowledgeResult(c, item, err, http.StatusOK)
}

func (h *Handler) requireClassifierStore(c *gin.Context) bool {
	if h.ClassifierStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "classifier store is not configured"})
		return false
	}
	return true
}

func writeClassifierError(c *gin.Context, err error) {
	switch err {
	case store.ErrConflict:
		c.JSON(http.StatusConflict, gin.H{"error": "active classifier cannot be deleted"})
	case store.ErrNotFound:
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	case store.ErrBadRequest:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
