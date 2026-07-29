package inventorysyncp9

import "github.com/gin-gonic/gin"

func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	api := g.Group("/inventory-sync")
	api.POST("/runs", h.CreateRun)
	api.GET("/runs", h.ListRuns)
	api.GET("/runs/:runId", h.GetRun)
	api.POST("/runs/:runId/rerun", h.Rerun)
	api.GET("/runs/:runId/snapshots", h.ListSnapshots)
	api.GET("/runs/:runId/audit-events", h.ListRunAudit)
	api.GET("/snapshots/:snapshotId", h.GetSnapshot)
	api.GET("/snapshots/:snapshotId/calibrations", h.ListCalibrations)
	api.POST("/snapshots/:snapshotId/recalibrate", h.Recalibrate)
	api.GET("/bindings", h.ListBindings)
	api.GET("/bindings/:bindingId", h.GetBinding)
	api.GET("/bindings/:bindingId/history", h.GetBindingHistory)
	api.GET("/manual-binding-requests", h.ListManualRequests)
	api.GET("/manual-binding-requests/:requestId", h.GetManualRequest)
	api.POST("/manual-binding-requests/:requestId/confirm", h.ConfirmManual)
	api.POST("/manual-binding-requests/:requestId/reject", h.RejectManual)
}
