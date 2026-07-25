// Command ptyshell is the M3 milestone demo: it puts the host terminal
// into raw mode, spawns the user's shell attached to a fresh pty, and
// copies bytes bidirectionally between the two — a minimal but
// complete "terminal inside a terminal," proving package pty's
// allocation, attach, resize propagation, and raw byte-forwarded
// signal handling end to end.
//
// This does no interpretation of the child's output — it's a raw
// passthrough. A real VT-emulator-backed Terminal widget that actually
// renders the child's screen comes later (M4/M10-M11).
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/sandgorgon/tui/pty"
	"github.com/sandgorgon/tui/term"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ptyshell:", err)
		os.Exit(1)
	}
}

func run() error {
	if !term.IsTerminal(os.Stdin) {
		return errors.New("stdin is not a terminal")
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	cmd := exec.Command(shell)

	fmt.Fprintf(os.Stderr, "ptyshell: attaching %s via a pty; Ctrl+D or 'exit' to quit\n", shell)

	p, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("pty.Start: %w", err)
	}
	defer p.Close()

	if size, err := term.GetSize(os.Stdout); err == nil {
		_ = p.Resize(size)
	}

	saved, err := term.MakeRaw(os.Stdin)
	if err != nil {
		return fmt.Errorf("MakeRaw: %w", err)
	}
	defer term.Restore(os.Stdin, saved)

	watcher := term.NewWatcher()
	defer watcher.Stop()
	go func() {
		for range watcher.Resize() {
			if size, err := term.GetSize(os.Stdout); err == nil {
				_ = p.Resize(size)
			}
		}
	}()
	go func() {
		for range watcher.Cont() {
			_, _ = term.MakeRaw(os.Stdin)
		}
	}()

	// stdin -> child: fire-and-forget; it naturally blocks reading
	// further keystrokes even after the child exits, and the process
	// exiting cleans it up.
	go func() { _, _ = io.Copy(p, os.Stdin) }()

	// child -> stdout: the one and only reader of p, so cmd.Wait()
	// completing and this goroutine's own EOF/error don't race.
	stdoutDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(os.Stdout, p)
		close(stdoutDone)
	}()

	waitErr := cmd.Wait()

	select {
	case <-stdoutDone:
	case <-time.After(300 * time.Millisecond):
		// The child exited but something (e.g. a lingering grandchild
		// still holding the slave open) kept the pty from EOFing;
		// don't hang the demo waiting for it.
	}

	if waitErr != nil {
		fmt.Fprintf(os.Stderr, "\r\nptyshell: %s exited: %v\r\n", shell, waitErr)
	}
	return nil
}
