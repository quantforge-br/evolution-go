package websocket_producer

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/evolution-foundation/evolution-go/pkg/config"
	logger_wrapper "github.com/evolution-foundation/evolution-go/pkg/logger"
	"github.com/gorilla/websocket"
)

func TestConcurrentEventsReachInstanceAndBroadcastClients(t *testing.T) {
	const count = 200
	manager := logger_wrapper.NewLoggerManager(&config.Config{LogDirectory: t.TempDir()})
	defer manager.GetLogger("test").Close()
	producer := NewWebsocketProducer(manager)
	registered := make(chan *websocket.Conn, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		if r.URL.Path == "/instance" {
			producer.AddClient("test", conn)
		} else {
			producer.AddBroadcastClient(conn)
		}
		registered <- conn
	}))
	defer server.Close()

	results := make(chan error, 2)
	for _, path := range []string{"/instance", "/broadcast"} {
		conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		defer (<-registered).Close()
		go func() {
			conn.SetReadDeadline(time.Now().Add(10 * time.Second))
			seen := make(map[string]bool)
			for range count {
				var message struct{ Queue, Payload string }
				if err := conn.ReadJSON(&message); err != nil {
					results <- err
					return
				}
				if message.Queue != "test.message" || seen[message.Payload] {
					results <- fmt.Errorf("invalid or duplicate event: %+v", message)
					return
				}
				seen[message.Payload] = true
			}
			results <- nil
		}()
	}

	start := make(chan struct{})
	var writers sync.WaitGroup
	for i := range count {
		writers.Add(1)
		go func() {
			defer writers.Done()
			<-start
			if err := producer.Produce("TEST.Message", []byte(fmt.Sprint(i)), "test", ""); err != nil {
				t.Error(err)
			}
		}()
	}
	close(start)
	writers.Wait()
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
}
