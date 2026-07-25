// Command multiplexer is the M6 milestone deliverable: a standalone,
// two-pane terminal multiplexer proving pty (M3), vt (M4), and render
// (M5) work together end to end before the component model (M8)
// exists to formalize this as a widget.
//
// Each pane runs $SHELL attached to its own pty; a vt.Parser/vt.Screen
// per pane interprets its output; every frame, both panes' screen
// buffers are composited into one host-sized cell.Buffer and handed to
// a render.Renderer, which diffs it against what's actually on the
// terminal and writes the minimal update. Ctrl+Left/Ctrl+Right switch
// focus between panes; the program exits once both shells have exited.
package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/render"
	"github.com/sandgorgon/tui/term"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "multiplexer:", err)
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

	hostSize, err := term.GetSize(os.Stdout)
	if err != nil {
		return fmt.Errorf("GetSize: %w", err)
	}

	leftRect, rightRect := computeLayout(hostSize.Cols, hostSize.Rows)
	leftPane, err := newPane(leftRect, shell)
	if err != nil {
		return fmt.Errorf("start left pane: %w", err)
	}
	rightPane, err := newPane(rightRect, shell)
	if err != nil {
		return fmt.Errorf("start right pane: %w", err)
	}
	panes := []*Pane{leftPane, rightPane}
	focused := 0

	saved, err := term.MakeRaw(os.Stdin)
	if err != nil {
		return fmt.Errorf("MakeRaw: %w", err)
	}
	defer term.Restore(os.Stdin, saved)

	// Alt screen on the *host* terminal too, so our redraws don't
	// clobber the user's real scrollback — the same courtesy vim/less
	// extend, now one level up.
	os.Stdout.WriteString("\x1b[?1049h")
	defer os.Stdout.WriteString("\x1b[?1049l")

	caps, leftover := term.Probe(os.Stdin, os.Stdout, 500*time.Millisecond, term.DetectEnv(os.Getenv))
	renderer := render.NewRenderer(render.Options{
		ColorLevel:         caps.ColorLevel,
		SynchronizedOutput: caps.SynchronizedOutput,
	})
	host := cell.NewBuffer(hostSize.Cols, hostSize.Rows)

	for _, p := range panes {
		go p.readLoop()
	}

	// Probe can leave bytes unread (a terminal's reply arriving late, or
	// the user's own early keystrokes) — route them to the focused pane
	// first, before real input starts flowing, or they'd be lost. This
	// is the fix for the bug where a terminal's own DA1/DECRQM reply
	// showed up as if typed into the left pane's shell.
	if len(leftover) > 0 {
		routeInput(leftover, panes, &focused)
	}

	exited := make(chan struct{})
	go func() {
		for _, p := range panes {
			_ = p.cmd.Wait()
		}
		close(exited)
	}()

	// term.Probe's own timeout is the primary defense against a
	// terminal's DA1/DECRQM reply leaking into input, but a slow or
	// batching terminal can still reply after Probe has already given
	// up — Probe genuinely never sees those bytes, so it has nothing to
	// return as leftover; they just arrive on a later, ordinary read.
	// Keep filtering reads for a further grace period to catch that
	// case too (see term.StripLateReply's doc comment).
	probeGraceDeadline := time.Now().Add(2 * time.Second)

	inputErr := make(chan error, 1)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				if time.Now().Before(probeGraceDeadline) {
					chunk = term.StripLateReply(chunk)
				}
				if len(chunk) > 0 {
					routeInput(chunk, panes, &focused)
				}
			}
			if err != nil {
				inputErr <- err
				return
			}
		}
	}()

	watcher := term.NewWatcher()
	defer watcher.Stop()

	redraw := func() {
		cx, cy, cvis := compositeFrame(host, panes, focused)
		_ = renderer.Render(os.Stdout, host, cx, cy, cvis)
	}
	redraw()

	// A fixed-rate redraw loop rather than change-driven scheduling:
	// render.Renderer's diff is a no-op when nothing changed, so this
	// is cheap when idle, and it sidesteps building the kind of
	// message/dirty-tracking loop that belongs to the real M8
	// component model, not this proof-of-concept.
	ticker := time.NewTicker(30 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			redraw()

		case <-watcher.Resize():
			hostSize, err = term.GetSize(os.Stdout)
			if err != nil {
				continue
			}
			leftRect, rightRect = computeLayout(hostSize.Cols, hostSize.Rows)
			leftPane.resize(leftRect)
			rightPane.resize(rightRect)
			host.Resize(hostSize.Cols, hostSize.Rows)
			redraw()

		case <-watcher.Cont():
			_, _ = term.MakeRaw(os.Stdin)
			redraw()

		case <-exited:
			return nil

		case <-inputErr:
			return nil
		}
	}
}
