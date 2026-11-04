package message_service

import (
	"testing"

	"go.mau.fi/whatsmeow/types"
)

func TestPollVoteTarget(t *testing.T) {
	own := types.NewADJID("5511999999999", 0, 7)
	cases := []struct {
		name    string
		data    PollVoteStruct
		creator string
		wantErr bool
	}{
		{"own poll uses our non-AD JID", PollVoteStruct{Number: "5522981218190", PollId: "P", FromMe: true, Options: []string{"A"}}, "5511999999999@s.whatsapp.net", false},
		{"direct chat poll is created by the chat", PollVoteStruct{Number: "5522981218190", PollId: "P", Options: []string{"A"}}, "5522981218190@s.whatsapp.net", false},
		{"group poll uses the participant", PollVoteStruct{Number: "120363000000000000@g.us", PollId: "P", Participant: "5522981218190", Options: []string{"A"}}, "5522981218190@s.whatsapp.net", false},
		{"group poll without participant", PollVoteStruct{Number: "120363000000000000@g.us", PollId: "P", Options: []string{"A"}}, "", true},
		{"missing poll id", PollVoteStruct{Number: "5522981218190", Options: []string{"A"}}, "", true},
		{"empty option name", PollVoteStruct{Number: "5522981218190", PollId: "P", Options: []string{""}}, "", true},
		{"withdrawing a vote is allowed", PollVoteStruct{Number: "5522981218190", PollId: "P", Options: nil}, "5522981218190@s.whatsapp.net", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, creator, _, err := pollVoteTarget(&c.data, own)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
			if !c.wantErr && creator.String() != c.creator {
				t.Fatalf("creator = %s, want %s", creator, c.creator)
			}
		})
	}
	_, _, opts, err := pollVoteTarget(&PollVoteStruct{Number: "5522981218190", PollId: "P", Options: []string{"A", "A", "B"}}, own)
	if err != nil || len(opts) != 2 {
		t.Fatalf("duplicates must collapse: %v %v", opts, err)
	}
}
