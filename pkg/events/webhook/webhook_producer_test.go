package webhook_producer

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebhookRejectsGlobalAndInstanceExternalDestinations(t *testing.T) {
	t.Setenv("LOCAL_HTTP_ALLOWED_HOSTS", "")
	producer := &webhookProducer{}
	for _, url := range []string{"https://webhook.site/global", "https://untrusted.invalid/instance"} {
		err, _, _ := producer.sendWebhook(url, []byte("private event"), "local-test")
		if err == nil || !strings.Contains(err.Error(), "privacy:") {
			t.Fatalf("external webhook was not blocked before DNS: %v", err)
		}
	}
}

func TestWebhookCanDeliverToLocalVPS(t *testing.T) {
	t.Setenv("LOCAL_HTTP_ALLOWED_HOSTS", "")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil || string(body) != "private event" || r.Method != http.MethodPost {
			t.Errorf("unexpected local webhook request: %q, %v", body, err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	producer := &webhookProducer{}
	if err, _, code := producer.sendWebhook(server.URL, []byte("private event"), "local-test"); err != nil || code != http.StatusNoContent {
		t.Fatalf("local webhook failed with status %d: %v", code, err)
	}
}
