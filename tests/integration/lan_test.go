package integration

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"testing"

	"deepseek-terminal/internal/client"
	"deepseek-terminal/internal/gateway"
	"deepseek-terminal/internal/provider/mock"
	"deepseek-terminal/internal/protocol"
	"deepseek-terminal/internal/session"
)

// findLANIPv4 returns the first non-loopback IPv4 address of this machine,
// or "" when none exists.
func findLANIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			if ip4 := ipnet.IP.To4(); ip4 != nil {
				return ip4.String()
			}
		}
	}
	return ""
}

// TestGatewayOverLANIPv4 validates the exact configuration of the Real
// Windows LAN test: the gateway bound to a LAN IPv4 address and a client
// connecting via that non-loopback address.
func TestGatewayOverLANIPv4(t *testing.T) {
	ip := findLANIPv4()
	if ip == "" {
		t.Skip("no non-loopback IPv4 interface available on this machine")
	}
	ln, err := net.Listen("tcp", ip+":0")
	if err != nil {
		t.Skipf("cannot bind to LAN IP %s: %v", ip, err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	mgr := session.New(session.Config{})
	gatewayURL := "http://" + ip + ":" + strconv.Itoa(port)
	g := gateway.New(gateway.Config{
		Provider:   &mock.Provider{},
		Model:      "deepseek-chat",
		Manager:    mgr,
		GatewayURL: gatewayURL,
	})
	srv := &http.Server{Handler: g.Handler()}
	defer srv.Close()
	go func() { _ = srv.Serve(ln) }()

	gc := client.NewGatewayClient(gatewayURL)

	// Full pairing lifecycle over the LAN-style address.
	sess, err := gc.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	st, err := gc.Status(sess.SessionID)
	if err != nil || st != protocol.StateWaiting {
		t.Fatalf("status = %q, %v", st, err)
	}
	if err := gc.Approve(sess.PairingToken); err != nil {
		t.Fatal(err)
	}
	if _, err := gc.Prompt(context.Background(), sess.SessionID, "hello from LAN", nil); err != nil {
		t.Fatal(err)
	}
	if err := gc.Close(sess.SessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := gc.Status(sess.SessionID); err == nil {
		t.Fatal("session still exists after close")
	}
}
