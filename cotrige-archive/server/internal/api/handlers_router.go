package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pangjian-pj/cotriage-dev/server/internal/store"
	redis "github.com/redis/go-redis/v9"
)

type routerStateResponse struct {
	Available bool            `json:"available"`
	Item      any             `json:"item,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

func (h *Handler) GetRouterSummary(c *gin.Context) {
	summary, err := h.routerStore().Summary(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error(), "degraded": true})
		return
	}
	enrichRouterSummaryFromRedis(c.Request.Context(), h, &summary)
	c.JSON(http.StatusOK, gin.H{"degraded": !h.routerStore().Available(), "summary": summary})
}

func (h *Handler) ListRouterDecisions(c *gin.Context) {
	limit := clampQueryInt(c, "limit", 50, 1, 200)
	offset := clampQueryInt(c, "offset", 0, 0, 10000)
	items, total, err := h.routerStore().ListDecisions(c.Request.Context(), store.RouterDecisionFilter{
		Action:     strings.TrimSpace(c.Query("action")),
		TemplateID: strings.TrimSpace(c.Query("template_id")),
		Source:     strings.TrimSpace(c.Query("source")),
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error(), "degraded": true, "items": []store.RouterDecisionModel{}, "total": 0})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "limit": limit, "offset": offset, "degraded": !h.routerStore().Available()})
}

func (h *Handler) GetRouterDecision(c *gin.Context) {
	eventID := strings.TrimSpace(c.Param("event_id"))
	if eventID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "event_id is required"})
		return
	}
	item, ok, err := h.routerStore().GetDecision(c.Request.Context(), eventID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error(), "degraded": true})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "router decision not found", "degraded": !h.routerStore().Available()})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) ListRouterOutbox(c *gin.Context) {
	limit := clampQueryInt(c, "limit", 50, 1, 200)
	offset := clampQueryInt(c, "offset", 0, 0, 10000)
	items, total, err := h.routerStore().ListOutbox(c.Request.Context(), strings.TrimSpace(c.Query("status")), limit, offset)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error(), "degraded": true, "items": []store.CloudOutboxModel{}, "total": 0})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "limit": limit, "offset": offset, "degraded": !h.routerStore().Available()})
}

func (h *Handler) ListCloudFeedback(c *gin.Context) {
	limit := clampQueryInt(c, "limit", 50, 1, 200)
	offset := clampQueryInt(c, "offset", 0, 0, 10000)
	items, total, err := h.routerStore().ListCloudFeedback(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error(), "degraded": true, "items": []store.CloudFeedbackModel{}, "total": 0})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "limit": limit, "offset": offset, "degraded": !h.routerStore().Available()})
}

func (h *Handler) GetRouterState(c *gin.Context) {
	snapshotType := strings.TrimSpace(c.DefaultQuery("type", ""))
	item, ok, err := h.routerStore().LatestState(c.Request.Context(), snapshotType)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error(), "degraded": true})
		return
	}
	if !ok {
		c.JSON(http.StatusOK, routerStateResponse{Available: false})
		return
	}
	payload := json.RawMessage(item.PayloadJSON)
	if !json.Valid(payload) {
		payload = nil
	}
	c.JSON(http.StatusOK, routerStateResponse{Available: true, Item: item, Payload: payload})
}

func (h *Handler) StreamRouterEvents(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	c.Writer.Flush()

	lastID := c.DefaultQuery("last_id", "$")
	for {
		streams, err := h.Redis.XRead(c.Request.Context(), &redis.XReadArgs{
			Streams: []string{h.Cfg.RoutedStream, lastID}, Count: 20, Block: 15 * time.Second,
		}).Result()
		if err == redis.Nil {
			_, _ = c.Writer.WriteString(": keepalive\n\n")
			c.Writer.Flush()
			continue
		}
		if err != nil {
			if errors.Is(err, context.Canceled) || c.Request.Context().Err() != nil {
				return
			}
			_, _ = c.Writer.WriteString("event: degraded\n")
			_, _ = c.Writer.WriteString("data: {\"error\":\"router stream unavailable\"}\n\n")
			c.Writer.Flush()
			return
		}
		for _, stream := range streams {
			for _, message := range stream.Messages {
				lastID = message.ID
				body, _ := json.Marshal(routerStreamPayload(message))
				_, _ = c.Writer.WriteString("id: " + message.ID + "\n")
				_, _ = c.Writer.WriteString("event: routed\n")
				_, _ = c.Writer.WriteString("data: " + string(body) + "\n\n")
				c.Writer.Flush()
			}
		}
	}
}

func (h *Handler) routerStore() *store.RouterStore {
	if h.RouterStore != nil {
		return h.RouterStore
	}
	return store.NewRouterStore(h.DB)
}

func enrichRouterSummaryFromRedis(ctx context.Context, h *Handler, summary *store.RouterSummary) {
	if h.Redis == nil {
		return
	}
	routedLen, _ := h.Redis.XLen(ctx, h.Cfg.RoutedStream).Result()
	cloudReqLen, _ := h.Redis.XLen(ctx, h.Cfg.CloudReqStream).Result()
	cloudRespLen, _ := h.Redis.XLen(ctx, h.Cfg.CloudRespStream).Result()
	if summary.RoutedTotal == 0 && routedLen > 0 {
		summary.RoutedTotal = routedLen
	}
	if summary.OutboxPending == 0 && cloudReqLen > cloudRespLen {
		summary.OutboxPending = cloudReqLen - cloudRespLen
	}
}

func routerStreamPayload(message redis.XMessage) any {
	if payload, ok := message.Values["payload"].(string); ok {
		var decoded any
		if err := json.Unmarshal([]byte(payload), &decoded); err == nil {
			return decoded
		}
		return gin.H{"payload": payload}
	}
	return message.Values
}

func clampQueryInt(c *gin.Context, key string, def, min, max int) int {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return def
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return def
	}
	if parsed < min {
		return min
	}
	if parsed > max {
		return max
	}
	return parsed
}
