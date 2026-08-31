package integration

import (
	"bytes"
	"context"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"deepseek-terminal/internal/client"
	"deepseek-terminal/internal/gateway"
	"deepseek-terminal/internal/protocol"
	"deepseek-terminal/internal/provider"
	"deepseek-terminal/internal/provider/mock"
	"deepseek-terminal/internal/session"
)

// autoApproveClient approves the session immediately after creation so the
// REPL pairing wait can be exercised against a real gateway.
type autoApproveClient struct {
	*client.GatewayClient
}

func (c *autoApproveClient) CreateSession() (protocol.SessionCreateResponse, error) {
	sess, err := c.GatewayClient.CreateSession()
	if err != nil {
		return sess, err
	}
	if err := c.GatewayClient.Approve(sess.PairingToken); err != nil {
		return sess, err
	}
	return sess, nil
}

func TestREPLAgainstRealGateway(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	mgr := session.New(session.Config{})
	mgr.StartSweeper(ctx)
	g := gateway.New(gateway.Config{
		Provider:   &mock.Provider{},
		Model:      string(provider.ModelV4Flash),
		Manager:    mgr,
		GatewayURL: "http://gw.local",
	})
	srv := httptest.NewServer(g.Handler())
	t.Cleanup(srv.Close)

	gc := client.NewGatewayClient(srv.URL)
	out := &bytes.Buffer{}
	repl := client.NewREPL(&autoApproveClient{gc}, strings.NewReader("hello\n/exit\n"), out)
	repl.SetQR(false)

	if err := repl.Run(ctx, make(chan os.Signal, 1)); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"Approved", "DeepSeek ready.", "mock reply to: hello", "Session destroyed."} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}
