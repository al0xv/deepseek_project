package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// WriteSSE writes one SSE frame: "event: {event}\ndata: {data}\n\n".
// data is JSON-encoded.
func WriteSSE(w io.Writer, event string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("sse: marshal %s: %w", event, err)
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b); err != nil {
		return fmt.Errorf("sse: write %s: %w", event, err)
	}
	return nil
}

// SSECallback receives the parsed event name and JSON payload.
// Returning an error stops reading.
type SSECallback func(event string, data []byte) error

// ReadSSE parses an SSE stream and dispatches each complete event.
func ReadSSE(r io.Reader, onEvent SSECallback) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var event string
	var dataLines []string

	flush := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		if err := onEvent(event, []byte(strings.Join(dataLines, "\n"))); err != nil {
			return err
		}
		event = ""
		dataLines = nil
		return nil
	}

	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			if err := flush(); err != nil {
				return err
			}
		case strings.HasPrefix(line, ":"):
			// comment line, ignore
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		default:
			// ignore other SSE fields (id, retry)
		}
	}
	if err := flush(); err != nil {
		return err
	}
	return sc.Err()
}
