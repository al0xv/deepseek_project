// Package client implements the terminal client: transport to the gateway
// and the interactive REPL.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"deepseek-terminal/internal/protocol"
)

// errStopReading is an internal sentinel that stops SSE reading on "done".
var errStopReading = errors.New("stop reading")

// Client is the interface the REPL needs from a remote gateway.
type Client interface {
	CreateSession() (protocol.SessionCreateResponse, error)
	Status(id string) (protocol.SessionState, error)
	Prompt(ctx context.Context, id, content string, onDelta func(string) error) (string, error)
	Cancel(id string) error
	Close(id string) error
}

// GatewayClient talks to a dsgateway over HTTP/SSE. It never holds the
// DeepSeek API key.
type GatewayClient struct {
	BaseURL string
	HTTP    *http.Client
}

// NewGatewayClient returns a client for baseURL (e.g. "http://localhost:8080").
func NewGatewayClient(baseURL string) *GatewayClient {
	return &GatewayClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		// No overall timeout: streaming responses are bounded by the
		// gateway's generation timeout, and cancellation is ctx-driven.
		HTTP: &http.Client{Timeout: 2 * time.Minute},
	}
}

// CreateSession implements Client.
func (c *GatewayClient) CreateSession() (protocol.SessionCreateResponse, error) {
	var out protocol.SessionCreateResponse
	err := c.doJSON(http.MethodPost, "/v1/sessions", nil, http.StatusCreated, &out)
	return out, err
}

// Status implements Client.
func (c *GatewayClient) Status(id string) (protocol.SessionState, error) {
	var out protocol.SessionStatusResponse
	err := c.doJSON(http.MethodGet, "/v1/sessions/"+id, nil, http.StatusOK, &out)
	if err != nil {
		return "", err
	}
	return out.State, nil
}

// Prompt implements Client. It streams token deltas to onDelta and returns
// the finish reason ("stop" | "length" | "cancelled").
func (c *GatewayClient) Prompt(ctx context.Context, id, content string, onDelta func(string) error) (string, error) {
	body, err := json.Marshal(protocol.PromptRequest{Content: content})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/sessions/"+id+"/prompt", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", readErrorBody(resp.Body, resp.StatusCode)
	}

	finish := ""
	err = protocol.ReadSSE(resp.Body, func(event string, data []byte) error {
		switch event {
		case "token":
			var t protocol.TokenEvent
			if err := json.Unmarshal(data, &t); err != nil {
				return err
			}
			if onDelta != nil {
				return onDelta(t.Delta)
			}
		case "done":
			var d protocol.DoneEvent
			if err := json.Unmarshal(data, &d); err != nil {
				return err
			}
			finish = d.FinishReason
			return errStopReading
		case "error":
			var e protocol.ErrorEvent
			if err := json.Unmarshal(data, &e); err != nil {
				return err
			}
			return fmt.Errorf("%s: %s", e.Code, e.Message)
		}
		return nil
	})
	if errors.Is(err, errStopReading) {
		err = nil
	}
	return finish, err
}

// Cancel implements Client: cancels the in-flight generation.
func (c *GatewayClient) Cancel(id string) error {
	return c.doNoContent(http.MethodPost, "/v1/sessions/"+id+"/cancel")
}

// Close implements Client: destroys the session.
func (c *GatewayClient) Close(id string) error {
	return c.doNoContent(http.MethodPost, "/v1/sessions/"+id+"/close")
}

// Approve approves a session with its pairing token (used by tests).
func (c *GatewayClient) Approve(token string) error {
	var out protocol.ApproveResponse
	return c.doJSON(http.MethodPost, "/v1/pair",
		protocol.ApproveRequest{PairingToken: token}, http.StatusOK, &out)
}

func (c *GatewayClient) doNoContent(method, path string) error {
	req, err := http.NewRequest(method, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return readErrorBody(resp.Body, resp.StatusCode)
	}
	return nil
}

func (c *GatewayClient) doJSON(method, path string, body any, wantStatus int, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		return readErrorBody(resp.Body, resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func readErrorBody(r io.Reader, status int) error {
	var eb protocol.ErrorBody
	_ = json.NewDecoder(io.LimitReader(r, 4096)).Decode(&eb)
	if eb.Code == "" {
		eb.Code = fmt.Sprintf("http_%d", status)
	}
	if eb.Message == "" {
		eb.Message = http.StatusText(status)
	}
	return fmt.Errorf("%s: %s", eb.Code, eb.Message)
}
