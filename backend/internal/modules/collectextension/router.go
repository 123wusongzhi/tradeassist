package collectextension

import "github.com/gin-gonic/gin"

// RegisterAdmin mounts Admin JWT-authenticated pairing and device management.
func RegisterAdmin(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.POST("/collect/browser-extension/pairings", h.CreatePairing)
	g.GET("/collect/browser-extension/devices", h.ListDevices)
	g.DELETE("/collect/browser-extension/devices/:id", h.RevokeDevice)
}

// RegisterPublic mounts pairing exchange plus the separately device-authenticated
// extension session/task endpoints.
func RegisterPublic(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.POST("/collect/browser-extension/pairings/exchange", h.ExchangePairing)
	device := g.Group("/collect/browser-extension")
	device.Use(h.DeviceAuth())
	device.GET("/session", h.Session)
	device.POST("/tasks", h.CreateTask)
	device.POST("/tasks/:id/result", h.SubmitResult)
	device.POST("/tasks/:id/failure", h.SubmitFailure)
}
