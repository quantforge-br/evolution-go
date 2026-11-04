package whatsmeow_service

import (
	"database/sql/driver"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/evolution-foundation/evolution-go/pkg/config"
	"github.com/evolution-foundation/evolution-go/pkg/history"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	logger_wrapper "github.com/evolution-foundation/evolution-go/pkg/logger"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

type historyDelivery struct{ delivered chan struct{} }

func (p *historyDelivery) Produce(string, []byte, string, string) error {
	p.delivered <- struct{}{}
	return nil
}
func (p *historyDelivery) CreateGlobalQueues() error { return nil }

type historyPayload struct {
	syncType waHistorySync.HistorySync_HistorySyncType
}

func (p historyPayload) Match(value driver.Value) bool {
	text, ok := value.(string)
	if !ok || strings.Contains(text, "test-secret") {
		return false
	}
	var event struct {
		Event string
		Data  struct {
			Data struct {
				SyncType int `json:"syncType"`
			}
		}
	}
	return json.Unmarshal([]byte(text), &event) == nil && event.Event == "HistorySync" && event.Data.Data.SyncType == int(p.syncType)
}

func TestInitialHistoryUsesPairedDeviceBeforeSettingsRefresh(t *testing.T) {
	for _, cachedOwner := range []string{"", "5511:7@s.whatsapp.net"} {
		name := "empty owner"
		if cachedOwner != "" {
			name = "previous device"
		}
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			cfg := &config.Config{LogDirectory: t.TempDir()}
			logs := logger_wrapper.NewLoggerManager(cfg)
			delivery := &historyDelivery{delivered: make(chan struct{}, 1)}
			service := &whatsmeowService{historyStore: history.New(db), loggerWrapper: logs, websocketProducer: delivery}
			device := types.NewADJID("5511", 0, 8)
			instance := &instance_model.Instance{Id: "instance", Jid: cachedOwner, Token: "test-secret", Events: "HISTORY_SYNC", WebSocketEnable: "true"}
			client := &MyClient{service: service, WAClient: &whatsmeow.Client{Store: &store.Device{ID: &device}}, Instance: instance, userID: "instance", token: "test-secret", config: cfg, loggerWrapper: logs}
			for _, syncType := range []waHistorySync.HistorySync_HistorySyncType{waHistorySync.HistorySync_INITIAL_BOOTSTRAP, waHistorySync.HistorySync_RECENT, waHistorySync.HistorySync_FULL} {
				mock.ExpectBegin()
				mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_xact_lock(hashtextextended($1, 0))")).WithArgs("instance/" + device.String()).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(regexp.QuoteMeta("INSERT INTO private_chat_events(instance_id, account_jid, payload) VALUES ($1, $2, $3)")).WithArgs("instance", device.String(), historyPayload{syncType}).WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
				client.myEventHandler(&events.HistorySync{Data: &waHistorySync.HistorySync{SyncType: syncType.Enum(), Progress: proto.Uint32(1)}})
				select {
				case <-delivery.delivered:
				case <-time.After(5 * time.Second):
					t.Fatal("history event was not delivered")
				}
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Fatal(err)
				}
			}
			if instance.Jid != cachedOwner {
				t.Fatal("dispatch mutated the shared instance")
			}
		})
	}
}
