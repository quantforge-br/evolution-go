package whatsmeow_service

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/evolution-foundation/evolution-go/pkg/config"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	instance_repository "github.com/evolution-foundation/evolution-go/pkg/instance/repository"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
)

func TestRetryConnectBacksOffUntilTheDialSucceeds(t *testing.T) {
	failures := 8
	calls := 0
	var waits []time.Duration
	connected := retryConnect(
		func() error {
			calls++
			if calls <= failures {
				return errors.New("lookup web.whatsapp.com: server misbehaving")
			}
			return nil
		},
		func() bool { return true },
		func(d time.Duration) { waits = append(waits, d) },
		func(int, time.Duration, error) {},
	)
	if !connected || calls != failures+1 {
		t.Fatalf("connected=%v after %d calls, want true after %d", connected, calls, failures+1)
	}
	want := []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second, 80 * time.Second, 160 * time.Second, 5 * time.Minute, 5 * time.Minute}
	if !reflect.DeepEqual(waits, want) {
		t.Fatalf("waits = %v, want %v", waits, want)
	}
}

func TestRetryConnectStopsOnceTheClientIsReplaced(t *testing.T) {
	calls := 0
	current := true
	connected := retryConnect(
		func() error { calls++; return errors.New("dial failed") },
		func() bool { return current },
		func(time.Duration) { current = false },
		func(int, time.Duration, error) {},
	)
	if connected || calls != 1 {
		t.Fatalf("connected=%v after %d calls, want false after 1", connected, calls)
	}

	connected = retryConnect(
		func() error { return nil },
		func() bool { return false },
		func(time.Duration) { t.Fatal("slept after a successful dial") },
		func(int, time.Duration, error) {},
	)
	if connected {
		t.Fatal("a replaced client reported a usable connection")
	}
}

type instancesByClient struct {
	instance_repository.InstanceRepository
	instances []*instance_model.Instance
}

func (r instancesByClient) GetAll(string) ([]*instance_model.Instance, error) {
	return r.instances, nil
}

func TestOfflineLinkedSessionsCountsOnlyLinkedDevices(t *testing.T) {
	device := types.NewADJID("5511", 0, 8)
	clients := map[string]*whatsmeow.Client{
		"linked":   {Store: &store.Device{ID: &device}},
		"pairing":  {Store: &store.Device{}},
		"no-store": {},
	}
	service := whatsmeowService{
		config:        &config.Config{ClientName: "evolution"},
		clientPointer: clients,
		instanceRepository: instancesByClient{instances: []*instance_model.Instance{
			{Id: "linked"}, {Id: "pairing"}, {Id: "no-store"}, {Id: "not-running"},
		}},
	}
	offline, err := service.OfflineLinkedSessions()
	if err != nil || offline != 1 {
		t.Fatalf("offline = %d, %v; want only the linked, disconnected device", offline, err)
	}

	delete(clients, "linked")
	if offline, _ := service.OfflineLinkedSessions(); offline != 0 {
		t.Fatalf("offline = %d without a linked client, want 0", offline)
	}
}
