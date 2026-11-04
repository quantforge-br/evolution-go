package send_service

import (
	"strings"
	"testing"
)

func TestExternalAudioConverterIsBlocked(t *testing.T) {
	t.Setenv("LOCAL_HTTP_ALLOWED_HOSTS", "")
	_, _, err := convertAudioWithApi("https://converter.invalid/audio", "private-key", ConvertAudio{})
	if err == nil || !strings.Contains(err.Error(), "privacy:") {
		t.Fatalf("external audio converter was not blocked: %v", err)
	}
}

func TestExternalLinkMetadataIsBlocked(t *testing.T) {
	t.Setenv("LOCAL_HTTP_ALLOWED_HOSTS", "")
	if _, _, _, err := fetchLinkMetadata("https://preview.invalid/private-link"); err == nil || !strings.Contains(err.Error(), "privacy:") {
		t.Fatalf("external link preview was not blocked: %v", err)
	}
}
