package whatsmeow_service

import (
	"context"
	"errors"
	"go.mau.fi/whatsmeow/types"
	"testing"
)

func TestOwnDeviceMessagesUseRecipientNotOwnSender(t *testing.T) {
	source := types.MessageSource{Chat: types.NewJID("99", types.HiddenUserServer), Sender: types.NewJID("11", types.HiddenUserServer), SenderAlt: types.NewJID("5511", types.DefaultUserServer), RecipientAlt: types.NewJID("5522", types.DefaultUserServer), IsFromMe: true}
	normalizeMessageChat(&source, nil)
	if source.Chat.String() != "5522@s.whatsapp.net" {
		t.Fatalf("wrong destination: %s", source.Chat)
	}
	source.Chat = types.NewJID("99", types.HiddenUserServer)
	source.RecipientAlt = types.EmptyJID
	normalizeMessageChat(&source, func(context.Context, types.JID) (types.JID, error) {
		return types.NewJID("5522", types.DefaultUserServer), nil
	})
	if source.Chat.String() != "5522@s.whatsapp.net" {
		t.Fatal("verified mapping was not used")
	}
	source.Chat = types.NewJID("99", types.HiddenUserServer)
	normalizeMessageChat(&source, func(context.Context, types.JID) (types.JID, error) { return types.EmptyJID, errors.New("unavailable") })
	if source.Chat.String() != "99@lid" {
		t.Fatal("must not guess from our own SenderAlt")
	}
	source.Chat = types.NewJID("120", types.GroupServer)
	source.IsGroup = true
	normalizeMessageChat(&source, nil)
	if source.Chat.String() != "120@g.us" {
		t.Fatal("group destination changed")
	}
}
