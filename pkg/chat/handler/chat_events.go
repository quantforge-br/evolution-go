package chat_handler

import (
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

// ChatEvents recovers persisted messages and receipts after desktop downtime.
// @Summary Get persisted chat events
// @Description Credential-free events scoped to the authenticated instance and paired device. Requires PRIVATE_CHAT_HISTORY=true.
// @Tags Chat
// @Produce json
// @Param after query string false "Last successfully persisted event ID"
// @Success 200 {object} gin.H
// @Failure 400 {object} gin.H
// @Failure 401 {object} gin.H
// @Failure 503 {object} gin.H
// @Router /chat/events [get]
func (c *chatHandler) ChatEvents(ctx *gin.Context) {
	value, exists := ctx.Get("instance")
	instance, ok := value.(*instance_model.Instance)
	if !exists || !ok || instance == nil {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authorized"})
		return
	}
	after, err := strconv.ParseInt(ctx.DefaultQuery("after", "0"), 10, 64)
	if err != nil || after < 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid history cursor"})
		return
	}
	page, err := c.chatService.ChatEvents(ctx.Request.Context(), instance, after)
	if err != nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "chat history storage unavailable"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": page})
}
