package chat_service

import (
	"context"
	"errors"
	"github.com/evolution-foundation/evolution-go/pkg/history"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	"go.mau.fi/whatsmeow/types"
)

var ErrInvalidRecovery = errors.New("invalid message recovery request")

type RecoveryRequest struct {
	Chat       string   `json:"chat"`
	MessageIDs []string `json:"messageIds"`
}

const recoveryQuery = `SELECT s.chat_jid, s.sender_jid FROM whatsmeow_message_secrets s
WHERE s.our_jid=$1 AND s.message_id=$2 AND (s.chat_jid=$3 OR s.chat_jid=(
 CASE WHEN $3 LIKE '%@s.whatsapp.net' THEN (SELECT lid || '@lid' FROM whatsmeow_lid_map WHERE pn=replace($3,'@s.whatsapp.net',''))
 WHEN $3 LIKE '%@lid' THEN (SELECT pn || '@s.whatsapp.net' FROM whatsmeow_lid_map WHERE lid=replace($3,'@lid','')) END)) LIMIT 1`

// RecoverMessages asks only our own primary device, never the conversation partner.
// IDs must already exist for this full device JID and verified conversation alias.
func (c *chatService) RecoverMessages(ctx context.Context, instance *instance_model.Instance, request RecoveryRequest) (int, error) {
	if instance == nil || c.chatSettingsDB == nil {
		return 0, history.ErrUnavailable
	}
	chat, err := types.ParseJID(request.Chat)
	if err != nil || chat.User == "" || (chat.Server != types.DefaultUserServer && chat.Server != types.HiddenUserServer && chat.Server != types.GroupServer) || len(request.MessageIDs) == 0 || len(request.MessageIDs) > 50 {
		return 0, ErrInvalidRecovery
	}
	type key struct {
		chat, sender types.JID
		id           string
	}
	keys := make([]key, 0, len(request.MessageIDs))
	seen := make(map[string]bool)
	for _, id := range request.MessageIDs {
		if len(id) == 0 || len(id) > 128 || seen[id] {
			return 0, ErrInvalidRecovery
		}
		seen[id] = true
		var storedChat, sender string
		if err := c.chatSettingsDB.QueryRowContext(ctx, recoveryQuery, instance.Jid, id, chat.ToNonAD().String()).Scan(&storedChat, &sender); err != nil {
			return 0, ErrInvalidRecovery
		}
		parsedChat, chatErr := types.ParseJID(storedChat)
		parsedSender, senderErr := types.ParseJID(sender)
		if chatErr != nil || senderErr != nil {
			return 0, ErrInvalidRecovery
		}
		keys = append(keys, key{parsedChat, parsedSender, id})
	}
	client := c.clientPointer[instance.Id]
	if client == nil || !client.IsConnected() || !client.IsLoggedIn() {
		return 0, errors.New("client disconnected")
	}
	// Each peer response is emitted as Message and enters the durable journal.
	for i, key := range keys {
		if _, err := client.SendPeerMessage(ctx, client.BuildUnavailableMessageRequest(key.chat, key.sender, key.id)); err != nil {
			return i, errors.New("phone recovery request failed")
		}
	}
	return len(keys), nil
}
