package history

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"regexp"
	"strings"
	"testing"
)

func TestAppendCommitsCredentialFreeOwnDeviceEventBeforeFanout(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	journal := New(db)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_xact_lock(hashtextextended($1, 0))")).WithArgs("instance/device:7@s.whatsapp.net").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(insertQuery)).WithArgs("instance", "device:7@s.whatsapp.net", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	data := json.RawMessage(`{"Info":{"IsFromMe":true},"Message":{"conversation":"sent from phone"}}`)
	if err := journal.Append(context.Background(), "instance", "device:7@s.whatsapp.net", "Message", data, nil); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestReadScopesByInstanceAndFullDevicePaginatesAndPreservesCursor(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	journal := New(db)
	mock.ExpectQuery(regexp.QuoteMeta(readQuery)).WithArgs("instance", "device:7@s.whatsapp.net", int64(5), 3).
		WillReturnRows(sqlmock.NewRows([]string{"id", "payload"}).AddRow(6, `{"event":"Message","data":{}}`).AddRow(8, `{"event":"Receipt","data":{}}`).AddRow(9, `{}`))
	page, err := journal.Read(context.Background(), "instance", "device:7@s.whatsapp.net", 5, 2)
	if err != nil || len(page.Events) != 2 || page.NextCursor != "8" || !page.HasMore {
		t.Fatalf("page: %+v, %v", page, err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(readQuery)).WithArgs("instance", "device:7@s.whatsapp.net", int64(8), 101).WillReturnRows(sqlmock.NewRows([]string{"id", "payload"}))
	page, err = journal.Read(context.Background(), "instance", "device:7@s.whatsapp.net", 8, 100)
	body, _ := json.Marshal(page)
	if err != nil || !strings.Contains(string(body), `"events":[]`) || page.NextCursor != "8" || page.HasMore {
		t.Fatalf("empty page: %s, %v", body, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestStorageFailureRollsBackAndRedactsDetails(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	journal := New(db)
	mock.ExpectBegin()
	mock.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnError(errors.New("password secret"))
	mock.ExpectRollback()
	if err := journal.Append(context.Background(), "instance", "device", "Message", json.RawMessage(`{}`), nil); err != ErrUnavailable {
		t.Fatalf("unsafe error: %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(readQuery)).WillReturnError(errors.New("password secret"))
	if _, err := journal.Read(context.Background(), "instance", "device", 0, 100); err != ErrUnavailable {
		t.Fatalf("unsafe error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
