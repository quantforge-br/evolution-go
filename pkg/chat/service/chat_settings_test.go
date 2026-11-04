package chat_service

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
)

func TestChatSettingsIsScopedToAuthenticatedDeviceAndWorksOffline(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := &chatService{chatSettingsDB: db}
	// Device ID must not be stripped: two instances can belong to the same phone.
	instance := &instance_model.Instance{Jid: "5511000000000:7@s.whatsapp.net", Token: "must-not-leak"}
	mock.ExpectQuery(regexp.QuoteMeta(chatSettingsQuery)).WithArgs(instance.Jid).
		WillReturnRows(sqlmock.NewRows([]string{"chat_jid", "archived", "pn"}).
			AddRow("120@g.us", true, "").AddRow("99@lid", false, "5511999999999"))
	settings, err := service.ChatSettings(context.Background(), instance)
	if err != nil {
		t.Fatal(err)
	}
	if len(settings) != 2 || !settings[0].Archived || settings[1].PhoneJID != "5511999999999@s.whatsapp.net" {
		t.Fatalf("unexpected settings: %+v", settings)
	}
	encoded, _ := json.Marshal(settings)
	if string(encoded) == "" {
		t.Fatal("empty response")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestChatSettingsEmptyIsAnArrayAndStorageFailuresAreSanitized(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := &chatService{chatSettingsDB: db}
	instance := &instance_model.Instance{Jid: "5511000000000:7@s.whatsapp.net"}
	mock.ExpectQuery(regexp.QuoteMeta(chatSettingsQuery)).WithArgs(instance.Jid).
		WillReturnRows(sqlmock.NewRows([]string{"chat_jid", "archived", "pn"}))
	settings, err := service.ChatSettings(context.Background(), instance)
	encoded, _ := json.Marshal(settings)
	if err != nil || string(encoded) != "[]" {
		t.Fatalf("empty settings: %s %v", encoded, err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(chatSettingsQuery)).WithArgs(instance.Jid).
		WillReturnError(errors.New("database password must-not-leak"))
	if _, err := service.ChatSettings(context.Background(), instance); !errors.Is(err, ErrChatSettingsUnavailable) {
		t.Fatalf("storage error must be sanitized: %v", err)
	}
	for _, instance := range []*instance_model.Instance{nil, {}, {Jid: "invalid"}, {Jid: "120@g.us"}} {
		if _, err := service.ChatSettings(context.Background(), instance); !errors.Is(err, ErrChatSettingsUnavailable) {
			t.Fatalf("invalid identity must fail closed: %v", err)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
