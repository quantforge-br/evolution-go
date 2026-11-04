package chat_service

import (
	"context"
	"errors"

	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	"go.mau.fi/whatsmeow/types"
)

var ErrChatSettingsUnavailable = errors.New("chat settings storage unavailable")

// ChatSetting is persisted app state, not a message or a request to the phone.
type ChatSetting struct {
	Chat     string `json:"chat"`
	Archived bool   `json:"archived"`
	PhoneJID string `json:"phoneJid,omitempty"`
}

const chatSettingsQuery = `SELECT c.chat_jid, c.archived, COALESCE(m.pn, '')
FROM whatsmeow_chat_settings c
LEFT JOIN whatsmeow_lid_map m ON c.chat_jid = m.lid || '@lid'
WHERE c.our_jid = $1 ORDER BY c.chat_jid`

// ChatSettings reads only the account belonging to the authenticated instance.
// It deliberately does not call ensureClientConnected or trigger a full app-state sync.
func (c *chatService) ChatSettings(ctx context.Context, instance *instance_model.Instance) ([]ChatSetting, error) {
	if c.chatSettingsDB == nil || instance == nil || instance.Jid == "" {
		return nil, ErrChatSettingsUnavailable
	}
	jid, err := types.ParseJID(instance.Jid)
	if err != nil || jid.User == "" || jid.Server != types.DefaultUserServer {
		return nil, ErrChatSettingsUnavailable
	}
	rows, err := c.chatSettingsDB.QueryContext(ctx, chatSettingsQuery, jid.String())
	if err != nil {
		return nil, ErrChatSettingsUnavailable
	}
	defer rows.Close()
	settings := make([]ChatSetting, 0)
	for rows.Next() {
		var setting ChatSetting
		if err := rows.Scan(&setting.Chat, &setting.Archived, &setting.PhoneJID); err != nil {
			return nil, ErrChatSettingsUnavailable
		}
		if setting.PhoneJID != "" {
			setting.PhoneJID += "@s.whatsapp.net"
		}
		settings = append(settings, setting)
	}
	if err := rows.Err(); err != nil {
		return nil, ErrChatSettingsUnavailable
	}
	return settings, nil
}
