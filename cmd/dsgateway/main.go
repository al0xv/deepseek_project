// Command dsgateway is the session gateway. It is the only component that
// holds the DeepSeek API key (or the mock provider in development).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"deepseek-terminal/internal/config"
	"deepseek-terminal/internal/gateway"
	"deepseek-terminal/internal/provider"
	"deepseek-terminal/internal/provider/deepseek"
	"deepseek-terminal/internal/provider/mock"
	"deepseek-terminal/internal/protocol"
	"deepseek-terminal/internal/session"
)

func main() {
	// The approve subcommand runs on the trusted machine against the
	// loopback gateway. It never transmits the API key.
	if len(os.Args) >= 2 && os.Args[1] == "approve" {
		runApprove(os.Args)
		return
	}

	mockMode := flag.Bool("mock", false, "use a built-in fake provider (no API key needed)")
	listen := flag.String("listen", "", `listen address, e.g. "0.0.0.0:8080" for LAN tests (default 127.0.0.1:8080)`)
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var p provider.Provider
	if *mockMode {
		p = &mock.Provider{}
		fmt.Println("dsgateway: using MOCK provider (no DeepSeek API key required)")
	} else {
		key, err := config.APIKey()
		if err != nil {
			fmt.Fprintln(os.Stderr, "dsgateway:", err)
			fmt.Fprintln(os.Stderr, "set DEEPSEEK_API_KEY or run with -mock")
			os.Exit(1)
		}
		dc := deepseek.New(key)
		dc.Model = config.Model()
		p = dc
	}

	addr := config.ListenAddr(*listen)
	gatewayURL := config.GatewayURL()
	genTimeout := config.Duration("DS_GEN_TIMEOUT", 60*time.Second)
	if !isLoopbackAddr(addr) {
		fmt.Printf("dsgateway: WARNING listening on %s — reachable from the LAN\n", addr)
	}

	mgr := session.New(session.Config{
		PairTimeout: config.Duration("DS_PAIR_TIMEOUT", 120*time.Second),
		IdleTimeout: config.Duration("DS_IDLE_TIMEOUT", 5*time.Minute),
		GenTimeout:  genTimeout,
		MaxSessions: config.Int("DS_MAX_SESSIONS", 8),
	})
	mgr.StartSweeper(ctx)

	gw := gateway.New(gateway.Config{
		Provider:   p,
		Model:      config.Model(),
		Manager:    mgr,
		GenTimeout: genTimeout,
		GatewayURL: gatewayURL,
	})

	srv := &http.Server{Addr: addr, Handler: gw.Handler()}
	go func() {
		fmt.Printf("dsgateway listening on %s (gateway URL: %s)\n", addr, gatewayURL)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, "dsgateway:", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	fmt.Println("dsgateway stopped.")
}

// isLoopbackAddr reports whether addr (host:port) binds only to loopback.
func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false // malformed or bare ":8080" — treat as non-loopback
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// runApprove implements `dsgateway approve <code>` on the trusted machine.
func runApprove(args []string) {
	if len(args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: dsgateway approve <code>")
		os.Exit(2)
	}
	code := protocol.NormalizePairCode(args[2])
	body, err := json.Marshal(protocol.ApproveRequest{PairingCode: code})
	if err != nil {
		fmt.Fprintln(os.Stderr, "approve:", err)
		os.Exit(1)
	}

	url := config.GatewayURL() + "/v1/pair"
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		fmt.Fprintln(os.Stderr, "approve:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var eb protocol.ErrorBody
		_ = json.NewDecoder(resp.Body).Decode(&eb)
		fmt.Fprintf(os.Stderr, "approve failed: %s: %s\n", eb.Code, eb.Message)
		os.Exit(1)
	}
	var ar protocol.ApproveResponse
	_ = json.NewDecoder(resp.Body).Decode(&ar)
	fmt.Printf("Approved session %s\n", ar.SessionID)
}
