package whatsmeow_service

import (
	"context"
	"go.mau.fi/whatsmeow/types"
)

// An own-device sender identifies us, not the destination of a direct chat.
// Only RecipientAlt or the verified LID store may resolve that destination.
func normalizeMessageChat(source *types.MessageSource, lookup func(context.Context, types.JID) (types.JID, error)) {
	if source.IsGroup || source.Chat.Server != types.HiddenUserServer {
		return
	}
	alternate := source.SenderAlt
	if source.IsFromMe {
		alternate = source.RecipientAlt
	}
	if alternate.Server == types.DefaultUserServer && alternate.User != "" {
		source.Chat = alternate.ToNonAD()
		return
	}
	if lookup != nil {
		if phone, err := lookup(context.Background(), source.Chat.ToNonAD()); err == nil && phone.Server == types.DefaultUserServer && phone.User != "" {
			source.Chat = phone.ToNonAD()
		}
	}
}
