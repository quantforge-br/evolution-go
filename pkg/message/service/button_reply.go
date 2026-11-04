package message_service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"

	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	"github.com/evolution-foundation/evolution-go/pkg/utils"
)

// maxButtonReplyField bounds ids and texts copied into the reply.
const maxButtonReplyField = 1024

// ButtonReplyStruct answers a message with reply buttons, a template or a list
// the way the official app does when a button or row is tapped. Type is the
// format of the message being answered: buttons, template, interactive (native
// flow quick_reply) or list.
type ButtonReplyStruct struct {
	Number      string `json:"number"`
	Id          string `json:"id"`
	Participant string `json:"participant,omitempty"`
	Type        string `json:"type"`
	ButtonId    string `json:"buttonId"`
	DisplayText string `json:"displayText"`
	Index       uint32 `json:"index,omitempty"`
	QuotedText  string `json:"quotedText,omitempty"`
}

// buildButtonReply validates the request and builds the reply message. The
// sender of the answered message is the chat itself in direct chats and must be
// given as participant in groups.
func buildButtonReply(data *ButtonReplyStruct) (chat types.JID, msg *waE2E.Message, msgType string, err error) {
	chat, ok := utils.ParseJID(data.Number)
	if !ok {
		return chat, nil, "", errors.New("invalid phone number")
	}
	chat = utils.CanonicalJID(chat)
	for name, value := range map[string]string{"id": data.Id, "buttonId": data.ButtonId, "displayText": data.DisplayText} {
		if strings.TrimSpace(value) == "" {
			return chat, nil, "", errors.New(name + " is required")
		}
		if len(value) > maxButtonReplyField {
			return chat, nil, "", errors.New(name + " is too long")
		}
	}
	if len(data.QuotedText) > 4*maxButtonReplyField {
		return chat, nil, "", errors.New("quotedText is too long")
	}
	sender := chat
	switch {
	case data.Participant != "":
		p, ok := utils.ParseJID(data.Participant)
		if !ok {
			return chat, nil, "", errors.New("invalid participant")
		}
		sender = utils.CanonicalJID(p).ToNonAD()
	case chat.Server == types.GroupServer:
		return chat, nil, "", errors.New("participant is required in groups")
	}
	// Same quote shape as the send routes: the answered message's id and sender,
	// with its text when the caller has it.
	quote := &waE2E.ContextInfo{
		StanzaID:      proto.String(data.Id),
		Participant:   proto.String(sender.String()),
		QuotedMessage: &waE2E.Message{Conversation: proto.String(data.QuotedText)},
	}
	switch data.Type {
	case "buttons":
		return chat, &waE2E.Message{ButtonsResponseMessage: &waE2E.ButtonsResponseMessage{
			SelectedButtonID: proto.String(data.ButtonId),
			Response:         &waE2E.ButtonsResponseMessage_SelectedDisplayText{SelectedDisplayText: data.DisplayText},
			Type:             waE2E.ButtonsResponseMessage_DISPLAY_TEXT.Enum(),
			ContextInfo:      quote,
		}}, "ButtonsResponseMessage", nil
	case "template":
		return chat, &waE2E.Message{TemplateButtonReplyMessage: &waE2E.TemplateButtonReplyMessage{
			SelectedID:          proto.String(data.ButtonId),
			SelectedDisplayText: proto.String(data.DisplayText),
			SelectedIndex:       proto.Uint32(data.Index),
			ContextInfo:         quote,
		}}, "TemplateButtonReplyMessage", nil
	case "interactive":
		params, _ := json.Marshal(map[string]string{"id": data.ButtonId, "display_text": data.DisplayText})
		return chat, &waE2E.Message{InteractiveResponseMessage: &waE2E.InteractiveResponseMessage{
			Body: &waE2E.InteractiveResponseMessage_Body{
				Text:   proto.String(data.DisplayText),
				Format: waE2E.InteractiveResponseMessage_Body_EXTENSIONS_1.Enum(),
			},
			InteractiveResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage_{
				NativeFlowResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage{
					Name:       proto.String("quick_reply"),
					ParamsJSON: proto.String(string(params)),
					Version:    proto.Int32(3),
				},
			},
			ContextInfo: quote,
		}}, "InteractiveResponseMessage", nil
	case "list":
		return chat, &waE2E.Message{ListResponseMessage: &waE2E.ListResponseMessage{
			Title:             proto.String(data.DisplayText),
			ListType:          waE2E.ListResponseMessage_SINGLE_SELECT.Enum(),
			SingleSelectReply: &waE2E.ListResponseMessage_SingleSelectReply{SelectedRowID: proto.String(data.ButtonId)},
			ContextInfo:       quote,
		}}, "ListResponseMessage", nil
	}
	return chat, nil, "", errors.New("type must be buttons, template, interactive or list")
}

// ButtonReply sends the reply to a tapped button or list row.
func (m *messageService) ButtonReply(data *ButtonReplyStruct, instance *instance_model.Instance) (*MessageSendStruct, error) {
	client, err := m.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}
	chat, msg, msgType, err := buildButtonReply(data)
	if err != nil {
		return nil, err
	}
	response, err := client.SendMessage(context.Background(), chat, msg)
	if err != nil {
		return nil, err
	}
	var own types.JID
	if client.Store.ID != nil {
		own = *client.Store.ID
	}
	return &MessageSendStruct{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{Chat: chat, Sender: own, IsFromMe: true, IsGroup: chat.Server == types.GroupServer},
			ID:            response.ID,
			Timestamp:     response.Timestamp,
			ServerID:      response.ServerID,
			Type:          msgType,
		},
		Message: msg,
	}, nil
}
