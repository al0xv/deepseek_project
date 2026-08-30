package client

import (
	"encoding/json"
	"fmt"
	"io"

	qrcode "github.com/skip2/go-qrcode"

	"deepseek-terminal/internal/protocol"
)

// pairingPayloadJSON renders the JSON payload that is encoded into the QR
// code. It never contains an API key.
func pairingPayloadJSON(s protocol.SessionCreateResponse) (string, error) {
	b, err := json.Marshal(protocol.PairingPayload{
		Version:      1,
		SessionID:    s.SessionID,
		PairingToken: s.PairingToken,
		GatewayURL:   s.GatewayURL,
	})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// renderQR writes an ASCII QR code for the pairing payload to w.
func renderQR(w io.Writer, s protocol.SessionCreateResponse) error {
	payload, err := pairingPayloadJSON(s)
	if err != nil {
		return err
	}
	qr, err := qrcode.New(payload, qrcode.Low)
	if err != nil {
		return fmt.Errorf("qr: %w", err)
	}
	_, err = io.WriteString(w, qr.ToString(false))
	return err
}
