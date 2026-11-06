package server_handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type ServerHandler interface {
	ServerOk(ctx *gin.Context)
	WhatsAppOk(ctx *gin.Context)
}

// SessionMonitor reports linked WhatsApp sessions that are not live.
type SessionMonitor interface {
	OfflineLinkedSessions() (int, error)
}

type serverHandler struct {
	sessions SessionMonitor
}

// ServerOk implements ServerHandler.
func (s *serverHandler) ServerOk(ctx *gin.Context) {
	ctx.JSON(200, gin.H{
		"status": "ok",
	})
}

// WhatsAppOk answers 503 while a linked device's WhatsApp session is down, so
// a health probe can tell "process up" from "WhatsApp connected". It must not
// back the API container's own healthcheck: Traefik drops routes to unhealthy
// containers, which would also cut the endpoints used to reconnect.
func (s *serverHandler) WhatsAppOk(ctx *gin.Context) {
	offline, err := s.sessions.OfflineLinkedSessions()
	switch {
	case err != nil:
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"status": "unknown"})
	case offline > 0:
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"status": "offline"})
	default:
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func NewServerHandler(sessions SessionMonitor) ServerHandler {
	return &serverHandler{sessions: sessions}
}
