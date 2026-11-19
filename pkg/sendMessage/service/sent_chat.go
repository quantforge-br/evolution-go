package send_service

import (
	"context"

	"go.mau.fi/whatsmeow/types"
)

// sentMessageSource reports a direct send under the contact's phone JID, as
// Message events do. IsOnWhatsApp answers with the LID, so the recipient of a
// send to a phone number is usually that LID; the LID moves to RecipientAlt.
func sentMessageSource(recipient, sender types.JID, isGroup bool, lookup func(context.Context, types.JID) (types.JID, error)) types.MessageSource {
	source := types.MessageSource{Chat: recipient, Sender: sender, IsFromMe: true, IsGroup: isGroup}
	if isGroup || recipient.Server != types.HiddenUserServer || lookup == nil {
		return source
	}
	lid := recipient.ToNonAD()
	if phone, err := lookup(context.Background(), lid); err == nil && phone.Server == types.DefaultUserServer && phone.User != "" {
		source.Chat = phone.ToNonAD()
		source.RecipientAlt = lid
	}
	return source
}
