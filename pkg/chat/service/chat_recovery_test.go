package chat_service

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	"regexp"
	"testing"
)

func TestRecoveryRequiresKnownIDsForAuthenticatedFullDeviceBeforeAnyPeerSend(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	service := &chatService{chatSettingsDB: db}
	owner := &instance_model.Instance{Id: "a", Jid: "5511:7@s.whatsapp.net"}
	request := RecoveryRequest{Chat: "5522@s.whatsapp.net", MessageIDs: []string{"known"}}
	mock.ExpectQuery(regexp.QuoteMeta(recoveryQuery)).WithArgs(owner.Jid, "known", request.Chat).
		WillReturnRows(sqlmock.NewRows([]string{"chat_jid", "sender_jid"}).AddRow("99@lid", "11@lid"))
	if _, err := service.RecoverMessages(context.Background(), owner, request); err == nil || err.Error() != "client disconnected" {
		t.Fatalf("expected offline failure after ownership validation, got %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(recoveryQuery)).WithArgs(owner.Jid, "foreign", request.Chat).
		WillReturnRows(sqlmock.NewRows([]string{"chat_jid", "sender_jid"}))
	request.MessageIDs = []string{"foreign"}
	if _, err := service.RecoverMessages(context.Background(), owner, request); err != ErrInvalidRecovery {
		t.Fatalf("unknown IDs accepted: %v", err)
	}
	for _, request := range []RecoveryRequest{{Chat: "status@broadcast", MessageIDs: []string{"known"}}, {Chat: "5522@s.whatsapp.net"}} {
		if _, err := service.RecoverMessages(context.Background(), owner, request); err != ErrInvalidRecovery {
			t.Fatalf("invalid request accepted: %v", err)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
