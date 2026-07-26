package operationtask

import "github.com/gin-gonic/gin"

func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	ot := g.Group("/operation-tasks")
	ot.POST("", h.CreateTask)
	ot.GET("", h.ListTasks)
	ot.GET("/:taskId", h.GetTask)
	ot.POST("/:taskId/cancel", h.CancelTask)
	ot.POST("/:taskId/drafts", h.CreateDraft)
	ot.PATCH("/:taskId/drafts/latest", h.EditLatestDraft)
	ot.GET("/:taskId/drafts", h.ListDrafts)
	ot.POST("/:taskId/approve", h.Approve)
	ot.POST("/:taskId/reject", h.Reject)
	ot.POST("/:taskId/execute", h.Execute)
	ot.POST("/:taskId/retry", h.Retry)
	ot.GET("/:taskId/attempts", h.ListAttempts)
	ot.GET("/:taskId/events", h.ListEvents)
}
