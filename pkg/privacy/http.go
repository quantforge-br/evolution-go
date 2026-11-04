// Package privacy restricts application HTTP integrations to explicitly trusted hosts.
// WhatsApp protocol traffic uses whatsmeow's own clients and is not intercepted here.
package privacy

import (
	"fmt"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"time"
)

// Reuse connections without forwarding local requests through environment proxies.
var localTransport = func() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return transport
}()

type restrictedTransport struct {
	base http.RoundTripper
}

func (t restrictedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Scheme != "http" && request.URL.Scheme != "https" {
		return nil, fmt.Errorf("privacy: unsupported HTTP scheme %q", request.URL.Scheme)
	}
	if !allowedHost(request.URL.Hostname(), os.Getenv("LOCAL_HTTP_ALLOWED_HOSTS")) {
		// Do not include query strings, credentials or payloads in the error.
		return nil, fmt.Errorf("privacy: outbound HTTP to host %q is blocked; use a VPS-local endpoint or explicitly configure LOCAL_HTTP_ALLOWED_HOSTS", request.URL.Hostname())
	}
	return t.base.RoundTrip(request)
}

func allowedHost(host, configuredHosts string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "localhost" {
		return true
	}
	if address, err := netip.ParseAddr(host); err == nil && address.Unmap().IsLoopback() {
		return true
	}
	for _, configured := range strings.Split(configuredHosts, ",") {
		configured = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(configured), "."))
		if configured != "" && host == configured {
			return true
		}
	}
	return false
}

// NewHTTPClient checks every request, including each redirect, before any DNS or
// HTTP connection. Host entries are exact names/IPs, not URLs or wildcard suffixes.
// Reading the allowlist at request time also supports .env loading in dev mode.
func NewHTTPClient() *http.Client {
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: restrictedTransport{base: localTransport},
	}
}

var client = NewHTTPClient()

// Get downloads media through the same allowlist used by webhooks and converters.
func Get(url string) (*http.Response, error) {
	return client.Get(url)
}
