package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWriteReadSSE(t *testing.T) {
	var buf strings.Builder
	if err := WriteSSE(&buf, "token", TokenEvent{Delta: "hi"}); err != nil {
		t.Fatal(err)
	}
	if err := WriteSSE(&buf, "done", DoneEvent{FinishReason: "stop"}); err != nil {
		t.Fatal(err)
	}

	var events []string
	var payloads []string
	err := ReadSSE(strings.NewReader(buf.String()), func(event string, data []byte) error {
		events = append(events, event)
		payloads = append(payloads, string(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0] != "token" || events[1] != "done" {
		t.Fatalf("events = %v", events)
	}
	var te TokenEvent
	if err := json.Unmarshal([]byte(payloads[0]), &te); err != nil {
		t.Fatal(err)
	}
	if te.Delta != "hi" {
		t.Fatalf("delta = %q", te.Delta)
	}
}

func TestReadSSEIgnoresCommentsAndMultilineData(t *testing.T) {
	input := ": comment\n" +
		"event: token\n" +
		"data: {\"delta\":\"a\"}\n" +
		"data: {\"delta\":\"b\"}\n\n"
	var got string
	err := ReadSSE(strings.NewReader(input), func(event string, data []byte) error {
		got = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Multiline data is joined with "\n" per SSE spec.
	if !strings.Contains(got, "a") || !strings.Contains(got, "b") {
		t.Fatalf("data = %q", got)
	}
}

func TestNormalizeAndFormatPairCode(t *testing.T) {
	if got := NormalizePairCode("472 913"); got != "472913" {
		t.Fatalf("normalize = %q", got)
	}
	if got := FormatPairCode("472913"); got != "472 913" {
		t.Fatalf("format = %q", got)
	}
}
