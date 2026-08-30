package client

import (
	"fmt"
	"io"

	qrcode "github.com/skip2/go-qrcode"

	"deepseek-terminal/internal/protocol"
)

// pairingPayloadString builds the QR payload for a session: a minimal URI
// carrying only the short-lived 6-digit pairing code. It deliberately does not
// include the gateway URL, session id, pairing token or any secret.
func pairingPayloadString(s protocol.SessionCreateResponse) string {
	return "dsremote://pair?v=1&code=" + protocol.NormalizePairCode(s.PairingCode)
}

// renderQR writes an ASCII QR code for the pairing payload to w.
func renderQR(w io.Writer, s protocol.SessionCreateResponse) error {
	payload := pairingPayloadString(s)
	qr, err := qrcode.New(payload, qrcode.Low)
	if err != nil {
		return fmt.Errorf("qr: %w", err)
	}
	_, err = io.WriteString(w, qr.ToString(false))
	return err
}
