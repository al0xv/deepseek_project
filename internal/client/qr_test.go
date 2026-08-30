package client

import (
	"strings"
	"testing"

	"deepseek-terminal/internal/protocol"
)

// TestPairingPayloadString pins the exact semantic payload placed in the QR
// code before rendering. The payload is a minimal URI with only the
// short-lived 6-digit pairing code.
func TestPairingPayloadString(t *testing.T) {
	s := protocol.SessionCreateResponse{
		SessionID:    "sess1",
		PairingToken: "tok1",
		PairingCode:  "472 913",
		GatewayURL:   "http://localhost:8080",
		ExpiresIn:    120,
	}
	got := pairingPayloadString(s)
	want := "dsremote://pair?v=1&code=472913"
	if got != want {
		t.Fatalf("payload = %q, want %q", got, want)
	}
	for _, forbidden := range []string{"sess1", "tok1", "localhost", "session_id", "pairing_token", "gateway_url", "api", "key"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("payload must not contain %q: %s", forbidden, got)
		}
	}
}

func TestRenderQRProducesBlockArt(t *testing.T) {
	s := protocol.SessionCreateResponse{
		SessionID:    "sess1",
		PairingToken: "tok1",
		PairingCode:  "472913",
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
