package client

import (
	"strings"
	"testing"

	"deepseek-terminal/internal/protocol"
)

func TestPairingPayloadJSON(t *testing.T) {
	s := protocol.SessionCreateResponse{
		SessionID:    "sess1",
		PairingToken: "tok1",
		GatewayURL:   "http://localhost:8080",
	}
	payload, err := pairingPayloadJSON(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"session_id":"sess1"`, `"pairing_token":"tok1"`, `"gateway_url":"http://localhost:8080"`, `"v":1`} {
		if !strings.Contains(payload, want) {
			t.Errorf("payload missing %q: %s", want, payload)
		}
	}
	if strings.Contains(payload, "api") || strings.Contains(payload, "key") {
		t.Errorf("payload must not contain key material: %s", payload)
	}
}

func TestRenderQRProducesBlockArt(t *testing.T) {
	s := protocol.SessionCreateResponse{
		SessionID:    "sess1",
		PairingToken: "tok1",
		GatewayURL:   "http://localhost:8080",
	}
	var sb strings.Builder
	if err := renderQR(&sb, s); err != nil {
		t.Fatal(err)
	}
	got := sb.String()
	if len(got) < 100 {
		t.Fatalf("QR too small (%d bytes): %q", len(got), got)
	}
	// QR block art uses block characters.
	if !strings.ContainsRune(got, '█') {
		t.Fatalf("QR does not contain block characters:\n%s", got)
	}
}
