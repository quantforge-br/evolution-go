package send_service

import (
	"context"
	"errors"
	"testing"

	"go.mau.fi/whatsmeow/types"
)

func TestSentMessagesUseThePhoneChatOfALIDRecipient(t *testing.T) {
	lid := types.NewJID("77", types.HiddenUserServer)
	own := types.JID{User: "5521", Device: 83, Server: types.DefaultUserServer}
	phone := func(context.Context, types.JID) (types.JID, error) {
		return types.NewJID("5544", types.DefaultUserServer), nil
	}
	source := sentMessageSource(lid, own, false, phone)
	if source.Chat.String() != "5544@s.whatsapp.net" || source.RecipientAlt.String() != "77@lid" {
		t.Fatalf("wrong chat %s / recipient alt %s", source.Chat, source.RecipientAlt)
	}
	if !source.IsFromMe || source.Sender != own {
		t.Fatal("sender changed")
	}
	unknown := func(context.Context, types.JID) (types.JID, error) { return types.EmptyJID, errors.New("unknown") }
	if source := sentMessageSource(lid, own, false, unknown); source.Chat != lid || !source.RecipientAlt.IsEmpty() {
		t.Fatal("must keep the LID without a verified mapping")
	}
	pn := types.NewJID("5544", types.DefaultUserServer)
	if source := sentMessageSource(pn, own, false, phone); source.Chat != pn || !source.RecipientAlt.IsEmpty() {
		t.Fatal("phone recipient changed")
	}
	group := types.NewJID("120", types.GroupServer)
	if source := sentMessageSource(group, own, true, phone); source.Chat != group {
		t.Fatal("group recipient changed")
	}
}
