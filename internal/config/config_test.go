package config

import "testing"

func TestGatewayAddrDefaultIsLoopback(t *testing.T) {
	t.Setenv("DS_GATEWAY_ADDR", "")
	if got := GatewayAddr(); got != "127.0.0.1:8080" {
		t.Fatalf("GatewayAddr = %q, want loopback default 127.0.0.1:8080", got)
	}
}

func TestGatewayAddrEnvOverride(t *testing.T) {
	t.Setenv("DS_GATEWAY_ADDR", "0.0.0.0:9999")
	if got := GatewayAddr(); got != "0.0.0.0:9999" {
		t.Fatalf("GatewayAddr = %q, want env value 0.0.0.0:9999", got)
	}
}

func TestListenAddrPrecedence(t *testing.T) {
	// 1. Explicit flag wins over env.
	t.Setenv("DS_GATEWAY_ADDR", "127.0.0.1:5000")
	if got := ListenAddr("0.0.0.0:8080"); got != "0.0.0.0:8080" {
		t.Fatalf("ListenAddr(flag) = %q, want flag value", got)
	}
	// 2. Env used when flag empty.
	if got := ListenAddr(""); got != "127.0.0.1:5000" {
		t.Fatalf("ListenAddr(empty) = %q, want env value", got)
	}
	// 3. Loopback default when nothing set.
	t.Setenv("DS_GATEWAY_ADDR", "")
	if got := ListenAddr(""); got != "127.0.0.1:8080" {
		t.Fatalf("ListenAddr(empty) = %q, want default", got)
	}
}

func TestGatewayURLDefault(t *testing.T) {
	t.Setenv("DS_GATEWAY_URL", "")
	if got := GatewayURL(); got != "http://localhost:8080" {
		t.Fatalf("GatewayURL = %q, want http://localhost:8080", got)
	}
}
