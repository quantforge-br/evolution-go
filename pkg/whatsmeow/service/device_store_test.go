package whatsmeow_service

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/evolution-foundation/evolution-go/pkg/config"
	"github.com/evolution-foundation/evolution-go/pkg/history"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	logger_wrapper "github.com/evolution-foundation/evolution-go/pkg/logger"
	"go.mau.fi/whatsmeow/store/sqlstore"
)

func TestRepeatedClientStartsUseTheSharedPoolAndLeaveHistoryAvailable(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	cfg := &config.Config{LogDirectory: t.TempDir()}
	service := whatsmeowService{
		config:        cfg,
		deviceStore:   sqlstore.NewWithDB(db, "postgres", nil),
		loggerWrapper: logger_wrapper.NewLoggerManager(cfg),
	}
	// More starts than PostgreSQL's usual 100-connection budget. A device
	// lookup failure must neither create a replacement pool nor close this one.
	for i := 0; i < 150; i++ {
		mock.ExpectQuery("SELECT .* FROM whatsmeow_device").
			WithArgs("5511:7@s.whatsapp.net").WillReturnError(errors.New("temporary device lookup failure"))
		service.StartClient(&ClientData{Instance: &instance_model.Instance{Id: "test", Jid: "5511:7@s.whatsapp.net"}})
	}
	mock.ExpectPing()
	if err := db.Ping(); err != nil {
		t.Fatalf("client failure closed the process-owned pool: %v", err)
	}
	mock.ExpectQuery("SELECT id, payload FROM private_chat_events").
		WithArgs("test", "5511:7@s.whatsapp.net", int64(0), 101).
		WillReturnRows(sqlmock.NewRows([]string{"id", "payload"}))
	if _, err := history.New(db).Read(context.Background(), "test", "5511:7@s.whatsapp.net", 0, 100); err != nil {
		t.Fatalf("history unavailable after repeated starts: %v", err)
	}
	if stats := db.Stats(); stats.MaxOpenConnections != 25 || stats.InUse != 0 {
		t.Fatalf("pool bounds or connections leaked: %+v", stats)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
