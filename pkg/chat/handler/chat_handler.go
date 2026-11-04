package chat_handler

import (
	"net/http"

	chat_service "github.com/evolution-foundation/evolution-go/pkg/chat/service"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	"github.com/gin-gonic/gin"
)

type ChatHandler interface {
	RecoverMessages(ctx *gin.Context)
	ChatEvents(ctx *gin.Context)
	ChatSettings(ctx *gin.Context)
	ChatPin(ctx *gin.Context)
	ChatUnpin(ctx *gin.Context)
	ChatArchive(ctx *gin.Context)
	ChatUnarchive(ctx *gin.Context)
	ChatMute(ctx *gin.Context)
	ChatUnmute(ctx *gin.Context)
	HistorySyncRequest(ctx *gin.Context)
}

type chatHandler struct {
	chatService chat_service.ChatService
}

// ChatSettings returns cached archive state without starting a WhatsApp session.
// @Summary Get stored chat archive settings
// @Description Read persisted archive state for the authenticated instance; no WhatsApp writes or reconnects.
// @Tags Chat
// @Produce json
// @Success 200 {object} gin.H
// @Failure 401 {object} gin.H
// @Failure 503 {object} gin.H
// @Router /chat/settings [get]
func (c *chatHandler) ChatSettings(ctx *gin.Context) {
	value, exists := ctx.Get("instance")
	instance, ok := value.(*instance_model.Instance)
	if !exists || !ok || instance == nil {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authorized"})
		return
	}
	settings, err := c.chatService.ChatSettings(ctx.Request.Context(), instance)
	if err != nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "chat settings storage unavailable"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": settings})
}

// Pin a chat
// @Summary Pin a chat
// @Description Pin a chat
// @Tags Chat
// @Accept json
// @Produce json
// @Param message body chat_service.BodyStruct true "Chat"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /chat/pin [post]
func (c *chatHandler) ChatPin(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *chat_service.BodyStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Chat == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "chat is required"})
		return
	}

	ts, err := c.chatService.ChatPin(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	responseData := gin.H{
		"timestamp": ts,
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": responseData})
}

// Unpin a chat
// @Summary Unpin a chat
// @Description Unpin a chat
// @Tags Chat
// @Accept json
// @Produce json
// @Param message body chat_service.BodyStruct true "Chat"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /chat/unpin [post]
func (c *chatHandler) ChatUnpin(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *chat_service.BodyStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Chat == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "chat is required"})
		return
	}

	ts, err := c.chatService.ChatUnpin(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	responseData := gin.H{
		"timestamp": ts,
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": responseData})
}

// Archive a chat
// @Summary Archive a chat
// @Description Archive a chat
// @Tags Chat
// @Accept json
// @Produce json
// @Param message body chat_service.BodyStruct true "Chat"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /chat/archive [post]
func (c *chatHandler) ChatArchive(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *chat_service.BodyStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Chat == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "chat is required"})
		return
	}

	ts, err := c.chatService.ChatArchive(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	responseData := gin.H{
		"timestamp": ts,
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": responseData})
}

// Unarchive a chat
// @Summary Unarchive a chat
// @Description Unarchive a chat
// @Tags Chat
// @Accept json
// @Produce json
// @Param message body chat_service.BodyStruct true "Chat"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /chat/unarchive [post]
func (c *chatHandler) ChatUnarchive(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *chat_service.BodyStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Chat == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "chat is required"})
		return
	}

	ts, err := c.chatService.ChatUnarchive(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	responseData := gin.H{
		"timestamp": ts,
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": responseData})
}

// Mute a chat
// @Summary Mute a chat
// @Description Mute a chat
// @Tags Chat
// @Accept json
// @Produce json
// @Param message body chat_service.BodyStruct true "Chat"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /chat/mute [post]
func (c *chatHandler) ChatMute(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *chat_service.BodyStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Chat == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "chat is required"})
		return
	}

	ts, err := c.chatService.ChatMute(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	responseData := gin.H{
		"timestamp": ts,
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": responseData})
}

// Unmute a chat
// @Summary Unmute a chat
// @Description Unmute a chat
// @Tags Chat
// @Accept json
// @Produce json
// @Param message body chat_service.BodyStruct true "Chat"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /chat/unmute [post]
func (c *chatHandler) ChatUnmute(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *chat_service.BodyStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Chat == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "chat is required"})
		return
	}

	ts, err := c.chatService.ChatUnmute(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	responseData := gin.H{
		"timestamp": ts,
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": responseData})
}

// HistorySyncRequest a chat
// @Summary HistorySyncRequest a chat
// @Description HistorySyncRequest a chat
// @Tags Chat
// @Accept json
// @Produce json
// @Param message body chat_service.HistorySyncRequestStruct true "Chat"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /chat/history-sync [post]
func (c *chatHandler) HistorySyncRequest(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *chat_service.HistorySyncRequestStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := c.chatService.HistorySyncRequest(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": resp})
}

func NewChatHandler(
	chatService chat_service.ChatService,
) ChatHandler {
	return &chatHandler{
		chatService: chatService,
	}
}
