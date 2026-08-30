package client

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"deepseek-terminal/internal/protocol"
)

// REPL implements the interactive terminal chat.
type REPL struct {
	client   Client
	in       io.Reader
	out      io.Writer
	enableQR bool

	sess      protocol.SessionCreateResponse
	streaming atomic.Bool
	cancelled atomic.Bool
}

// NewREPL returns a REPL that reads from in and writes to out.
func NewREPL(c Client, in io.Reader, out io.Writer) *REPL {
	return &REPL{client: c, in: in, out: out, enableQR: true}
}

// SetQR enables or disables QR rendering (tests and non-TTY terminals).
func (r *REPL) SetQR(enabled bool) {
	r.enableQR = enabled
}

// Run performs the full session lifecycle: pairing, chat, teardown.
// sig receives SIGINT notifications (Ctrl+C). It returns when the session
// ends or an unrecoverable error occurs.
func (r *REPL) Run(ctx context.Context, sig chan os.Signal) error {
	// 1. Create the session.
	sess, err := r.client.CreateSession()
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	r.sess = sess

	// 2. Show pairing instructions.
	fmt.Fprintln(r.out, "New session created.")
	fmt.Fprintln(r.out, "Pairing required to continue.")
	fmt.Fprintln(r.out)
	fmt.Fprintln(r.out, "  WAITING")
	if r.enableQR {
		if err := renderQR(r.out, sess); err != nil {
			fmt.Fprintf(r.out, "  (QR unavailable: %v)\n", err)
		}
	}
	fmt.Fprintf(r.out, "  Code: %s\n\n", sess.PairingCode)
	fmt.Fprintf(r.out, "Waiting for approval... (expires in %ds)\n", sess.ExpiresIn)

	// 3. Wait for approval.
	if err := r.waitApproved(ctx, sig); err != nil {
		_ = r.client.Close(r.sess.SessionID)
		fmt.Fprintln(r.out)
		fmt.Fprintf(r.out, "%v\n", err)
		fmt.Fprintln(r.out, "Session destroyed.")
		return err
	}

	fmt.Fprintln(r.out)
	fmt.Fprintln(r.out, "✓ Approved")
	fmt.Fprintln(r.out)
	fmt.Fprintln(r.out, "DeepSeek ready.")
	fmt.Fprintln(r.out)

	// 4. Interrupt handling during the chat loop:
	//    - during streaming: first Ctrl+C cancels the generation;
	//    - at the prompt: Ctrl+C ends the session.
	quit := make(chan struct{})
	var quitOnce sync.Once
	go func() {
		for {
			select {
			case <-ctx.Done():
				quitOnce.Do(func() { close(quit) })
				return
			case <-sig:
				if r.streaming.Load() {
					if !r.cancelled.Swap(true) {
						_ = r.client.Cancel(r.sess.SessionID)
					}
				} else {
					quitOnce.Do(func() { close(quit) })
				}
			}
		}
	}()

	// 5. Chat loop.
	lines := make(chan string, 1)
	go readLines(r.in, lines)

	for {
		fmt.Fprint(r.out, "You > ")
		select {
		case <-quit:
			r.teardown()
			return nil
		case line, ok := <-lines:
			if !ok {
				// EOF (Ctrl+D / Ctrl+Z).
				r.teardown()
				return nil
			}
			switch {
			case line == "/exit":
				r.teardown()
				return nil
			case line == "":
				continue
			default:
				r.doPrompt(ctx, line)
			}
		}
	}
}

func (r *REPL) waitApproved(ctx context.Context, sig chan os.Signal) error {
	deadline := time.Now().Add(time.Duration(r.sess.ExpiresIn) * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("cancelled")
		case <-sig:
			return fmt.Errorf("cancelled by user")
		case <-ticker.C:
			state, err := r.client.Status(r.sess.SessionID)
			if err == nil {
				switch state {
				case protocol.StateApproved:
					return nil
				case protocol.StateDestroyed:
					return fmt.Errorf("session destroyed")
				}
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("pairing expired")
			}
		}
	}
}

func (r *REPL) doPrompt(ctx context.Context, content string) {
	r.cancelled.Store(false)
	r.streaming.Store(true)
	defer r.streaming.Store(false)

	fmt.Fprintf(r.out, "DeepSeek > ")
	finish, err := r.client.Prompt(ctx, r.sess.SessionID, content, func(delta string) error {
		_, werr := io.WriteString(r.out, delta)
		return werr
	})
	if err != nil {
		fmt.Fprintf(r.out, "\nError: %v\n", err)
		return
	}
	fmt.Fprintln(r.out)
	if finish == "cancelled" {
		fmt.Fprintln(r.out, "[cancelled]")
	}
}

func (r *REPL) teardown() {
	_ = r.client.Close(r.sess.SessionID)
	fmt.Fprintln(r.out, "Session destroyed.")
}

func readLines(in io.Reader, out chan<- string) {
	br := bufio.NewReader(in)
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			if line != "" {
				out <- strings.TrimSpace(line)
			}
			close(out)
			return
		}
		out <- strings.TrimSpace(line)
	}
}
