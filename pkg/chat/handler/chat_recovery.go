package chat_handler

import (
	"errors"
	chat_service "github.com/evolution-foundation/evolution-go/pkg/chat/service"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	"github.com/gin-gonic/gin"
	"net/http"
)

// RecoverMessages requests known missing messages from the paired phone.
// @Summary Recover known messages from own phone
// @Tags Chat
// @Accept json
// @Produce json
// @Param request body chat_service.RecoveryRequest true "Known message IDs and chat"
// @Success 200 {object} gin.H
// @Failure 400 {object} gin.H
// @Failure 401 {object} gin.H
// @Failure 503 {object} gin.H
// @Router /chat/history-recover [post]
func (c *chatHandler) RecoverMessages(ctx *gin.Context) {
	value, exists := ctx.Get("instance")
	instance, ok := value.(*instance_model.Instance)
	if !exists || !ok || instance == nil {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authorized"})
		return
	}
	var request chat_service.RecoveryRequest
	if ctx.ShouldBindJSON(&request) != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid message recovery request"})
		return
	}
	count, err := c.chatService.RecoverMessages(ctx.Request.Context(), instance, request)
	if err != nil {
		status := http.StatusServiceUnavailable
		if errors.Is(err, chat_service.ErrInvalidRecovery) {
			status = http.StatusBadRequest
		}
		ctx.JSON(status, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": gin.H{"requested": count}})
}
