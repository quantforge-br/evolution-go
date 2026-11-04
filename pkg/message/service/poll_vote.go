package message_service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"

	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	poll_model "github.com/evolution-foundation/evolution-go/pkg/poll/model"
	"github.com/evolution-foundation/evolution-go/pkg/utils"
)

// maxPollVoteOptions matches WhatsApp's limit of 12 options per poll.
const maxPollVoteOptions = 12

// PollVoteStruct votes on a poll. Options are option names exactly as in the
// poll; an empty list withdraws the vote.
type PollVoteStruct struct {
	Number      string   `json:"number"`
	PollId      string   `json:"pollId"`
	FromMe      bool     `json:"fromMe"`
	Participant string   `json:"participant,omitempty"`
	Options     []string `json:"options"`
}

// pollVoteTarget resolves the poll's chat and creator. The creator is needed to
// look up the poll's message secret, which encrypts the vote.
func pollVoteTarget(data *PollVoteStruct, own types.JID) (chat, creator types.JID, options []string, err error) {
	if strings.TrimSpace(data.PollId) == "" {
		return chat, creator, nil, errors.New("pollId is required")
	}
	chat, ok := utils.ParseJID(data.Number)
	if !ok {
		return chat, creator, nil, errors.New("invalid phone number")
	}
	chat = utils.CanonicalJID(chat)
	isGroup := chat.Server == types.GroupServer
	switch {
	case data.FromMe:
		if own.IsEmpty() {
			return chat, creator, nil, errors.New("instance is not logged in")
		}
		creator = own.ToNonAD()
	case data.Participant != "":
		p, ok := utils.ParseJID(data.Participant)
		if !ok {
			return chat, creator, nil, errors.New("invalid participant")
		}
		creator = utils.CanonicalJID(p).ToNonAD()
	case isGroup:
		return chat, creator, nil, errors.New("participant is required for polls created by others in groups")
	default:
		creator = chat
	}
	seen := map[string]bool{}
	for _, o := range data.Options {
		if o == "" {
			return chat, creator, nil, errors.New("options must not be empty strings")
		}
		if !seen[o] {
			seen[o] = true
			options = append(options, o)
		}
	}
	if len(options) > maxPollVoteOptions {
		return chat, creator, nil, fmt.Errorf("at most %d options can be selected", maxPollVoteOptions)
	}
	return chat, creator, options, nil
}

// PollVote sends an encrypted poll vote and records it with the received votes,
// since WhatsApp does not echo our own votes back as events.
func (m *messageService) PollVote(data *PollVoteStruct, instance *instance_model.Instance) (*MessageSendStruct, error) {
	client, err := m.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}
	var own types.JID
	if client.Store.ID != nil {
		own = *client.Store.ID
	}
	chat, creator, options, err := pollVoteTarget(data, own)
	if err != nil {
		return nil, err
	}
	isGroup := chat.Server == types.GroupServer
	pollInfo := &types.MessageInfo{
		MessageSource: types.MessageSource{Chat: chat, Sender: creator, IsFromMe: data.FromMe, IsGroup: isGroup},
		ID:            data.PollId,
	}
	ctx := context.Background()
	msg, err := client.BuildPollVote(ctx, pollInfo, options)
	if err != nil {
		if errors.Is(err, whatsmeow.ErrOriginalMessageSecretNotFound) {
			return nil, errors.New("poll secret not found: this device never received the poll")
		}
		return nil, err
	}
	response, err := client.SendMessage(ctx, chat, msg)
	if err != nil {
		return nil, err
	}

	if pollService := m.whatsmeowService.GetPollService(); pollService != nil {
		selected := make([]string, 0, len(options))
		for _, hash := range whatsmeow.HashPollOptions(options) {
			selected = append(selected, fmt.Sprintf("%x", hash))
		}
		voter := own.ToNonAD()
		vote := &poll_model.PollVote{
			ID:              uuid.New().String(),
			InstanceID:      instance.Id,
			PollMessageID:   data.PollId,
			PollChatJid:     chat.String(),
			VoteMessageID:   response.ID,
			VoterJid:        voter.String(),
			VoterPhone:      voter.User,
			VoterName:       client.Store.PushName,
			SelectedOptions: selected,
			VotedAt:         response.Timestamp,
			ReceivedAt:      time.Now(),
		}
		saveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := pollService.SavePollVote(saveCtx, vote); err != nil {
			m.loggerWrapper.GetLogger(instance.Id).LogWarn("[%s] Poll vote sent but not recorded: %v", instance.Id, err)
		}
	}

	return &MessageSendStruct{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{Chat: chat, Sender: own, IsFromMe: true, IsGroup: isGroup},
			ID:            response.ID,
			Timestamp:     response.Timestamp,
			ServerID:      response.ServerID,
			Type:          "PollUpdateMessage",
		},
		Message: msg,
	}, nil
}
