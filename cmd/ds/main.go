// Command ds is the DeepSeek terminal client. It never holds the API key.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"deepseek-terminal/internal/client"
	"deepseek-terminal/internal/config"
)

func main() {
	remote := flag.String("remote", "", "gateway base URL (default from DS_GATEWAY_URL)")
	flag.Parse()

	baseURL := *remote
	if baseURL == "" {
		baseURL = config.GatewayURL()
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)

	ctx := context.Background()
	gc := client.NewGatewayClient(baseURL)
	repl := client.NewREPL(gc, os.Stdin, os.Stdout)

	fmt.Println("DeepSeek Terminal")
	fmt.Println("-----------------")
	fmt.Println()

	if err := repl.Run(ctx, sig); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
