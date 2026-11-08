package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/evolution-foundation/evolution-go/pkg/config"
	"github.com/gin-gonic/gin"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestLocalRouterPreservesAuthenticationWithoutActivation(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		GlobalApiKey: "private-admin-key",
		ClientName:   "local-test",
		LogDirectory: t.TempDir(),
	}
	router := setupRouter(db, sqlDB, nil, nil, cfg, nil, t.TempDir())
	for _, key := range []string{"", "wrong-key"} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/instance/all", nil)
		request.Header.Set("apikey", key)
		router.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("invalid API key returned %d, want 401", response.Code)
		}
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "instances" WHERE client_name = $1`)).
		WithArgs(cfg.ClientName).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/instance/all", nil)
	request.Header.Set("apikey", cfg.GlobalApiKey)
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("authenticated local request returned %d: %s", response.Code, response.Body)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestChatSettingsRouteUsesInstanceTokenAndIgnoresAccountOverrides(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{GlobalApiKey: "admin-key", ClientName: "local-test", LogDirectory: t.TempDir()}
	router := setupRouter(db, sqlDB, nil, nil, cfg, nil, t.TempDir())
	check := func(key string, code int) string {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/chat/settings?instanceId=another-account&jid=other@s.whatsapp.net", nil)
		request.Header.Set("apikey", key)
		router.ServeHTTP(response, request)
		if response.Code != code {
			t.Fatalf("status %d, want %d: %s", response.Code, code, response.Body)
		}
		return response.Body.String()
	}
	check("", http.StatusUnauthorized)
	authQuery := regexp.QuoteMeta("SELECT * FROM \"instances\" WHERE token = $1")
	for _, key := range []string{"admin-key", "invalid-token"} {
		mock.ExpectQuery(authQuery).WithArgs(key, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id"}))
		check(key, http.StatusUnauthorized)
	}
	for _, account := range []struct{ token, jid, chat string }{
		{"instance-a", "5511000000000:1@s.whatsapp.net", "1201@g.us"},
		{"instance-b", "5511000000000:2@s.whatsapp.net", "1202@g.us"},
	} {
		mock.ExpectQuery(authQuery).WithArgs(account.token, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "jid", "token"}).AddRow(account.token, account.jid, account.token))
		mock.ExpectQuery(regexp.QuoteMeta("SELECT c.chat_jid, c.archived, COALESCE")).WithArgs(account.jid).
			WillReturnRows(sqlmock.NewRows([]string{"chat_jid", "archived", "pn"}).AddRow(account.chat, true, ""))
		body := check(account.token, http.StatusOK)
		if !strings.Contains(body, account.chat) || strings.Contains(body, account.token) || strings.Contains(body, account.jid) {
			t.Fatalf("unexpected response: %s", body)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestChatEventsRouteAuthenticatesAndScopesFullDeviceWithoutQueryOverrides(t *testing.T) {
	t.Setenv("PRIVATE_CHAT_HISTORY", "true")
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS private_chat_events").WillReturnResult(sqlmock.NewResult(0, 0))
	cfg := &config.Config{GlobalApiKey: "admin-key", ClientName: "local-test", LogDirectory: t.TempDir()}
	router := setupRouter(db, sqlDB, nil, nil, cfg, nil, t.TempDir())
	check := func(key, after string, code int) string {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/chat/events?after="+after+"&instanceId=other&jid=other", nil)
		request.Header.Set("apikey", key)
		router.ServeHTTP(response, request)
		if response.Code != code {
			t.Fatalf("status %d want %d: %s", response.Code, code, response.Body)
		}
		return response.Body.String()
	}
	check("", "0", http.StatusUnauthorized)
	authQuery := regexp.QuoteMeta("SELECT * FROM \"instances\" WHERE token = $1")
	mock.ExpectQuery(authQuery).WithArgs("admin-key", 1).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	check("admin-key", "0", http.StatusUnauthorized)
	for _, owner := range []struct{ token, jid string }{{"a", "5511:1@s.whatsapp.net"}, {"b", "5511:2@s.whatsapp.net"}} {
		mock.ExpectQuery(authQuery).WithArgs(owner.token, 1).WillReturnRows(sqlmock.NewRows([]string{"id", "jid", "token"}).AddRow(owner.token, owner.jid, owner.token))
		mock.ExpectQuery("SELECT id, payload FROM private_chat_events WHERE instance_id=\\$1 AND account_jid=\\$2").WithArgs(owner.token, owner.jid, int64(5), 101).
			WillReturnRows(sqlmock.NewRows([]string{"id", "payload"}).AddRow(8, []byte("{\"event\":\"Message\",\"data\":{\"Info\":{\"IsFromMe\":true}}}")))
		body := check(owner.token, "5", http.StatusOK)
		if !strings.Contains(body, "\"nextCursor\":\"8\"") || strings.Contains(body, "instanceToken") || strings.Contains(body, owner.jid) {
			t.Fatalf("unsafe or wrong response: %s", body)
		}
	}
	mock.ExpectQuery(authQuery).WithArgs("a", 1).WillReturnRows(sqlmock.NewRows([]string{"id", "jid", "token"}).AddRow("a", "5511:1@s.whatsapp.net", "a"))
	check("a", "-1", http.StatusBadRequest)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAccessLogMasksCredentialQueryParameters(t *testing.T) {
	var logged bytes.Buffer
	previous := gin.DefaultWriter
	gin.DefaultWriter = &logged
	t.Cleanup(func() { gin.DefaultWriter = previous })
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{GlobalApiKey: "admin-key", ClientName: "local-test", LogDirectory: t.TempDir()}
	router := setupRouter(db, sqlDB, nil, nil, cfg, nil, t.TempDir())
	for _, target := range []string{"/ws?token=leaked-admin-key&instanceId=inst1", "/server/ok?ApiKey=leaked-instance-token"} {
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, target, nil))
	}
	log := logged.String()
	if strings.Contains(log, "leaked-") {
		t.Fatalf("access log exposes a credential:\n%s", log)
	}
	for _, want := range []string{"/ws?token=<redacted>&instanceId=inst1", "/server/ok?ApiKey=<redacted>"} {
		if !strings.Contains(log, want) {
			t.Fatalf("access log lacks %q:\n%s", want, log)
		}
	}
}

func TestDeviceDBBorrowsTheBoundedAuthPoolAcrossClientRestarts(t *testing.T) {
	authDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer authDB.Close()
	authDB.SetMaxOpenConns(25)
	for i := 0; i < 150; i++ {
		db, dialect, err := initDeviceDB(authDB, t.TempDir())
		if err != nil || db != authDB || dialect != "postgres" {
			t.Fatalf("device pool = %p, %q, %v; want existing AUTH pool %p", db, dialect, err, authDB)
		}
		if db.Stats().MaxOpenConnections != 25 {
			t.Fatal("device store removed the AUTH pool's connection limit")
		}
	}
}

func TestSQLiteDeviceDBUsesOneBoundedPool(t *testing.T) {
	db, dialect, err := initDeviceDB(nil, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if dialect != "sqlite" || db.Stats().MaxOpenConnections != 1 {
		t.Fatalf("unbounded or wrong SQLite pool: %q, %+v", dialect, db.Stats())
	}
	shared := sqlstore.NewWithDB(db, dialect, nil)
	if err := shared.Upgrade(context.Background()); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 150; i++ {
		if _, err := shared.GetAllDevices(context.Background()); err != nil {
			t.Fatalf("shared SQLite store failed on reuse %d: %v", i, err)
		}
	}
	if stats := db.Stats(); stats.MaxOpenConnections != 1 || stats.InUse != 0 {
		t.Fatalf("SQLite connections leaked: %+v", stats)
	}
}
