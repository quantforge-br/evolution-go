package instance_service

import (
	"testing"

	"github.com/evolution-foundation/evolution-go/pkg/config"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	instance_repository "github.com/evolution-foundation/evolution-go/pkg/instance/repository"
	logger_wrapper "github.com/evolution-foundation/evolution-go/pkg/logger"
	whatsmeow_service "github.com/evolution-foundation/evolution-go/pkg/whatsmeow/service"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type oneInstance struct {
	instance_repository.InstanceRepository
	instance *instance_model.Instance
}

func (r oneInstance) GetInstanceByID(string) (*instance_model.Instance, error) {
	return r.instance, nil
}

type recordedStart struct {
	whatsmeow_service.WhatsmeowService
	started chan *whatsmeow_service.ClientData
}

func (s recordedStart) ForceUpdateJid(string, string) error { return nil }

func (s recordedStart) StartClient(cd *whatsmeow_service.ClientData) { s.started <- cd }

// Logout, Disconnect and the LoggedOut handler send on killChannel[id]; a
// missing channel makes them block forever once a forced reconnect replaced a
// stuck client.
func TestForceReconnectLeavesAKillChannelForTheNewClient(t *testing.T) {
	const id = "instance"
	cfg := &config.Config{LogDirectory: t.TempDir()}
	kill := map[string]chan bool{}
	clients := map[string]*whatsmeow.Client{id: whatsmeow.NewClient(&store.Device{}, waLog.Noop)}
	starts := recordedStart{started: make(chan *whatsmeow_service.ClientData, 1)}
	service := NewInstanceService(oneInstance{instance: &instance_model.Instance{Id: id}}, kill, clients, starts, cfg, logger_wrapper.NewLoggerManager(cfg))

	_ = service.ForceReconnect(id, "5511") // the fake never connects a client
	if cd := <-starts.started; cd.Instance.Id != id {
		t.Fatalf("started %q, want %q", cd.Instance.Id, id)
	}
	if kill[id] == nil {
		t.Fatal("no kill channel left for the restarted client")
	}
}
