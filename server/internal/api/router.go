package api

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, h *Handler) {
	api := r.Group("/api/v1")
	api.GET("/devices", h.ListDevices)
	api.GET("/dashboard/summary", h.GetDashboardSummary)
	api.GET("/dashboard/timeseries", h.GetDashboardTimeseries)
	api.GET("/dashboard/abnormal-analysis", h.GetAbnormalAnalysis)
	api.GET("/competition-metrics", h.GetCompetitionMetrics)
	api.GET("/competition-metrics/history", h.ListCompetitionMetricSnapshots)
	api.GET("/events", h.ListEvents)
	api.GET("/events/stream", h.StreamEvents)
	api.GET("/router/summary", h.GetRouterSummary)
	api.GET("/router/decisions", h.ListRouterDecisions)
	api.GET("/router/decisions/:event_id", h.GetRouterDecision)
	api.GET("/router/outbox", h.ListRouterOutbox)
	api.GET("/router/cloud-feedback", h.ListCloudFeedback)
	api.GET("/router/state", h.GetRouterState)
	api.GET("/router/stream", h.StreamRouterEvents)

	api.POST("/edge-providers", h.CreateEdgeProvider)
	api.PATCH("/edge-providers/:id", h.PatchEdgeProvider)
	api.GET("/edge-providers/:id", h.GetEdgeProvider)
	api.GET("/edge-providers", h.ListEdgeProviders)
	api.DELETE("/edge-providers/:id", h.DeleteEdgeProvider)
	api.POST("/edge-providers/:id/activate", h.ActivateEdgeProvider)

	api.POST("/edge-classifiers", h.CreateClassifierConfig)
	api.PATCH("/edge-classifiers/:id", h.PatchClassifierConfig)
	api.GET("/edge-classifiers", h.ListClassifierConfigs)
	api.DELETE("/edge-classifiers/:id", h.DeleteClassifierConfig)
	api.POST("/edge-classifiers/:id/activate", h.ActivateClassifierConfig)

	api.POST("/edge-rules", h.CreateEdgeRule)
	api.PATCH("/edge-rules/:id", h.PatchEdgeRule)
	api.GET("/edge-rules", h.ListEdgeRules)
	api.DELETE("/edge-rules/:id", h.DeleteEdgeRule)
	api.POST("/edge-rule-sets", h.CreateEdgeRuleSet)
	api.POST("/edge-rule-sets/:id/activate", h.ActivateEdgeRuleSet)

	api.POST("/edge-templates", h.CreateEdgeTemplate)
	api.PATCH("/edge-templates/:id", h.PatchEdgeTemplate)
	api.GET("/edge-templates", h.ListEdgeTemplates)
	api.POST("/edge-templates/from-cluster", h.CreateEdgeTemplateFromCluster)
	api.DELETE("/edge-templates/:id", h.DeleteEdgeTemplate)
	api.POST("/edge-template-sets", h.CreateEdgeTemplateSet)
	api.POST("/edge-template-sets/:id/activate", h.ActivateEdgeTemplateSet)

	api.GET("/edge-config", h.GetEdgeConfig)

	api.POST("/cloud-providers", h.CreateCloudProvider)
	api.GET("/cloud-providers", h.ListCloudProviders)
	api.GET("/cloud-providers/:id", h.GetCloudProvider)
	api.PATCH("/cloud-providers/:id", h.PatchCloudProvider)
	api.DELETE("/cloud-providers/:id", h.DeleteCloudProvider)
	api.POST("/cloud-providers/:id/activate", h.ActivateCloudProvider)
	api.GET("/cloud-config", h.GetCloudConfig)
}
