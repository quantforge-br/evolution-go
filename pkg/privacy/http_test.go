package privacy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAllowedHost(t *testing.T) {
	tests := []struct {
		name, host, configured string
		want                   bool
	}{
		{"localhost", "localhost", "", true},
		{"IPv4 loopback", "127.0.0.1", "", true},
		{"IPv6 loopback", "::1", "", true},
		{"mapped loopback", "::ffff:127.0.0.1", "", true},
		{"exact host", "media", "media,converter", true},
		{"normalized host", "MEDIA.EXAMPLE.", " media.example. ", true},
		{"exact IP", "172.20.0.3", "172.20.0.3", true},
		{"unlisted private IP", "10.0.0.2", "", false},
		{"vendor license", "license.evolutionfoundation.com.br", "", false},
		{"vendor telemetry", "log.evolution-api.com", "", false},
		{"external media", "picsum.photos", "", false},
		{"suffix attack", "media.example.evil.test", "media.example", false},
		{"localhost suffix", "localhost.evil.test", "", false},
		{"wildcard rejected", "media.example", "*.example", false},
		{"URL rejected", "media.example", "https://media.example", false},
		{"empty host", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := allowedHost(tt.host, tt.configured); got != tt.want {
				t.Fatalf("allowedHost(%q, %q) = %v, want %v", tt.host, tt.configured, got, tt.want)
			}
		})
	}
}

type recordingTransport struct{ calls int }

func (r *recordingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	r.calls++
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("ok")),
		Request:    request,
	}, nil
}

func TestRestrictedTransportChecksBeforeNetwork(t *testing.T) {
	t.Setenv("LOCAL_HTTP_ALLOWED_HOSTS", "media")
	base := &recordingTransport{}
	transport := restrictedTransport{base: base}
	for _, url := range []string{
		"https://license.evolutionfoundation.com.br/v1/activate",
		"https://log.evolution-api.com/telemetry",
		"https://untrusted.test/?token=secret",
		"ftp://localhost/file",
	} {
		request, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := transport.RoundTrip(request); err == nil {
			t.Fatalf("expected %q to be blocked", url)
		} else if strings.Contains(err.Error(), "token=secret") {
			t.Fatalf("error leaked query string: %v", err)
		}
	}
	if base.calls != 0 {
		t.Fatalf("blocked requests reached the underlying transport %d times", base.calls)
	}
	request := httptest.NewRequest(http.MethodGet, "http://media/file", nil)
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if base.calls != 1 {
		t.Fatalf("expected one explicitly allowed request, got %d", base.calls)
	}
}

func TestLocalGetAndRedirectPolicy(t *testing.T) {
	t.Setenv("LOCAL_HTTP_ALLOWED_HOSTS", "")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "https://blocked.invalid/media", http.StatusFound)
			return
		}
		io.WriteString(w, "local media")
	}))
	defer server.Close()

	response, err := Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || string(body) != "local media" {
		t.Fatalf("unexpected local response %q: %v", body, err)
	}
	if _, err := NewHTTPClient().Get(server.URL + "/redirect"); err == nil || !strings.Contains(err.Error(), "privacy:") {
		t.Fatalf("expected external redirect to be blocked before DNS, got %v", err)
	}
	if localTransport.Proxy != nil {
		t.Fatal("local integration transport must not forward data through environment proxies")
	}
}
