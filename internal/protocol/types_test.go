package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestTerminalProtocolHasNoControlToken guards the invariant that the
// controller capability never appears in messages sent to the untrusted
// terminal client.
func TestTerminalProtocolHasNoControlToken(t *testing.T) {
	cases := []struct {
		name string
		body any
	}{
		{"SessionCreateResponse", SessionCreateResponse{
			SessionID: "s", PairingToken: "t", PairingCode: "472 913",
			GatewayURL: "http://gw", ExpiresIn: 120,
		}},
		{"SessionStatusResponse", SessionStatusResponse{SessionID: "s", State: StateApproved}},
		{"PromptRequest", PromptRequest{Content: "hi"}},
	}
	for _, c := range cases {
		b, err := json.Marshal(c.body)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if strings.Contains(string(b), "control_token") {
			t.Fatalf("%s must not contain control_token: %s", c.name, b)
		}
	}
}

func TestApproveResponseCarriesControlToken(t *testing.T) {
	b, err := json.Marshal(ApproveResponse{SessionID: "s", State: StateApproved, ControlToken: "tok123"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"control_token":"tok123"`) {
		t.Fatalf("approve response missing control_token: %s", b)
	}
}
