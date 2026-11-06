package whatsmeow_service

import (
	"errors"
	"reflect"
	"testing"
	"time"
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
