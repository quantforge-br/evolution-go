package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
)

type forbiddenTransport struct{ calls atomic.Int64 }

func (t *forbiddenTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	t.calls.Add(1)
	return nil, fmt.Errorf("unexpected outbound request to %s", request.URL.Host)
}

func TestLocalLicenseRoutesNeverContactRemoteServices(t *testing.T) {
	transport := &forbiddenTransport{}
	originalTransport, originalClient := http.DefaultTransport, http.DefaultClient
	http.DefaultTransport = transport
	http.DefaultClient = &http.Client{Transport: transport}
	t.Cleanup(func() {
		http.DefaultTransport, http.DefaultClient = originalTransport, originalClient
	})
	t.Setenv("EVOLUTION_OPERATOR_EMAIL", "must-not-be-sent@example.com")

	router := gin.New()
	LicenseRoutes(router)
	for _, path := range []string{
		"/license/status",
		"/license/register?redirect_uri=https://untrusted.invalid/callback",
		"/license/activate?code=must-not-be-exchanged",
		"/license/activate",
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("apikey", "must-not-be-returned")
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s returned %d: %s", path, response.Code, response.Body)
		}
		var result map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result["status"] != "active" || result["mode"] != "local" || result["telemetry_enabled"] != false {
			t.Fatalf("unexpected local status: %#v", result)
		}
		for _, key := range []string{"api_key", "instance_id", "register_url", "email", "customer_id"} {
			if _, found := result[key]; found {
				t.Fatalf("%s exposed %q", path, key)
			}
		}
	}
	if calls := transport.calls.Load(); calls != 0 {
		t.Fatalf("local compatibility routes made %d outbound requests", calls)
	}
}
