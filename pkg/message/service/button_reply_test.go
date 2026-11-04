package message_service

import (
	"encoding/json"
	"strings"
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
)

func reply(t string) *ButtonReplyStruct {
	return &ButtonReplyStruct{Number: "5511999990000", Id: "BTN1", Type: t, ButtonId: "yes-id", DisplayText: "Yes", Index: 1, QuotedText: "Continue?"}
}

func TestButtonReplyFormats(t *testing.T) {
	_, msg, typ, err := buildButtonReply(reply("buttons"))
	if err != nil || typ != "ButtonsResponseMessage" {
		t.Fatalf("buttons: %v %s", err, typ)
	}
	b := msg.GetButtonsResponseMessage()
	if b.GetSelectedButtonID() != "yes-id" || b.GetSelectedDisplayText() != "Yes" || b.GetType() != waE2E.ButtonsResponseMessage_DISPLAY_TEXT {
		t.Fatalf("buttons reply: %+v", b)
	}
	ci := b.GetContextInfo()
	if ci.GetStanzaID() != "BTN1" || ci.GetParticipant() != "5511999990000@s.whatsapp.net" || ci.GetQuotedMessage().GetConversation() != "Continue?" {
		t.Fatalf("quote: %+v", ci)
	}

	_, msg, _, err = buildButtonReply(reply("template"))
	tr := msg.GetTemplateButtonReplyMessage()
	if err != nil || tr.GetSelectedID() != "yes-id" || tr.GetSelectedIndex() != 1 || tr.GetSelectedDisplayText() != "Yes" {
		t.Fatalf("template: %v %+v", err, tr)
	}

	_, msg, _, err = buildButtonReply(reply("interactive"))
	nf := msg.GetInteractiveResponseMessage().GetNativeFlowResponseMessage()
	var params map[string]string
	if err != nil || nf.GetName() != "quick_reply" || json.Unmarshal([]byte(nf.GetParamsJSON()), &params) != nil || params["id"] != "yes-id" {
		t.Fatalf("interactive: %v %+v", err, nf)
	}
	if msg.GetInteractiveResponseMessage().GetBody().GetText() != "Yes" {
		t.Fatal("interactive body")
	}

	_, msg, _, err = buildButtonReply(reply("list"))
	lr := msg.GetListResponseMessage()
	if err != nil || lr.GetSingleSelectReply().GetSelectedRowID() != "yes-id" || lr.GetTitle() != "Yes" {
		t.Fatalf("list: %v %+v", err, lr)
	}
}

func TestButtonReplyValidation(t *testing.T) {
	bad := []func(*ButtonReplyStruct){
		func(d *ButtonReplyStruct) { d.Type = "flow" },
		func(d *ButtonReplyStruct) { d.Id = " " },
		func(d *ButtonReplyStruct) { d.ButtonId = "" },
		func(d *ButtonReplyStruct) { d.DisplayText = "" },
		func(d *ButtonReplyStruct) { d.ButtonId = strings.Repeat("x", maxButtonReplyField+1) },
		func(d *ButtonReplyStruct) { d.Number = "120363000000000000@g.us" },
		func(d *ButtonReplyStruct) { d.Number = "not a number" },
	}
	for i, mutate := range bad {
		d := reply("buttons")
		mutate(d)
		if _, _, _, err := buildButtonReply(d); err == nil {
			t.Errorf("case %d: expected an error", i)
		}
	}
	d := reply("buttons")
	d.Number = "120363000000000000@g.us"
	d.Participant = "5511888880000@s.whatsapp.net"
	_, msg, _, err := buildButtonReply(d)
	if err != nil || msg.GetButtonsResponseMessage().GetContextInfo().GetParticipant() != "5511888880000@s.whatsapp.net" {
		t.Fatalf("group reply: %v", err)
	}
}
