package widget

import (
	"os/exec"
	"sync"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/pty"
	"github.com/sandgorgon/tui/term"
	"github.com/sandgorgon/tui/tui"
	"github.com/sandgorgon/tui/vt"
)

// TerminalOptions configures Terminal.
type TerminalOptions struct {
	// Command starts a fresh pty-attached child the first time this
	// Node mounts — read once, like TextInput's Value: a Terminal owns
	// its child process for its whole retained lifetime, not something
	// restarted or resynced from props on every frame.
	Command *exec.Cmd

	// OnExit, if non-nil, is called after the child process exits
	// (err is nil on a clean exit) the next time HandleEvent runs —
	// see Terminal's doc comment for why it isn't delivered the instant
	// the child actually exits.
	OnExit func(err error) tui.Msg

	// WantsRawTab, if true, claims Tab for the child process (e.g.
	// shell tab-completion) instead of App's global Tab focus
	// navigation (see tui.RawKeyClaimer) — off by default, so existing
	// Terminal usage is unaffected. ReleaseKey is what exits the pane
	// and resumes navigation instead; unlike TextArea, Esc is not a
	// safe default here — a real shell or editor running inside needs
	// Esc delivered to it (vim's insert-mode exit, readline bindings,
	// ...) — so when WantsRawTab is true and ReleaseKey is left at its
	// zero value, it defaults to Ctrl+\, which is rarely bound by
	// shells or editors in practice. Set it explicitly to whatever key
	// your application reserves for "leave this pane."
	WantsRawTab bool
	ReleaseKey  input.KeyEvent
}

// Terminal wires package pty (L6) and package vt (L7) together as a
// normal retained widget: pty output bytes feed a vt.Parser, which
// mutates a vt.Screen; Paint blits that screen's cells straight into
// the Painter's rect (the same technique
// examples/multiplexer/compositor.go used by hand against a raw
// cell.Buffer, now via Painter.SetRawCell); HandleEvent does the
// reverse, encoding key/mouse input.Events back into bytes written to
// the pty master (see terminal_encode.go — the reverse of package
// input's Decoder, which until now only ever needed to decode).
// This is the formalization of M6's standalone multiplexer prototype
// into the real widget docs/DESIGN.md §5 always called for.
//
// A Terminal owns real OS resources — the pty master fd and a
// background goroutine reading it — for as long as its retained
// Widget instance exists; it implements io.Closer so those are
// released the moment its Node stops appearing in the tree (see
// tui/dispose.go).
//
// Two known, deliberate limitations, both consequences of this
// library's change-driven redraw model (docs/DESIGN.md §8, the same
// tradeoff M6's ticker-based prototype existed specifically to avoid
// needing): the pty's output updates Terminal's internal vt.Screen
// state continuously in the background, but that only becomes visible
// the next time the App happens to render a frame for any reason — an
// app hosting a Terminal that wants live-updating output while
// otherwise idle needs to drive its own periodic redraw (e.g. a self-
// rescheduling Tick Cmd). And OnExit fires opportunistically, from
// HandleEvent, rather than the instant the child exits — a retained
// Widget has no channel of its own into the App's Cmd/Msg loop (only
// Model.Update/HandleEvent can originate a Cmd), so there's no way to
// push a notification the moment a background goroutine notices the
// child died; the next keystroke (or any other event) picks it up.
func Terminal(opts TerminalOptions) tui.Node {
	return tui.Component(nil, opts, func() tui.Widget {
		return &terminalWidget{}
	})
}

type terminalWidget struct {
	opts    TerminalOptions
	mounted bool

	pty *pty.Pty
	cmd *exec.Cmd

	mu      sync.Mutex
	parser  *vt.Parser
	screen  *vt.Screen
	exited  bool
	exitErr error

	lastCols, lastRows int
	focused            bool
}

func (w *terminalWidget) Reconcile(props any) bool {
	w.opts = props.(TerminalOptions)
	return true
}

// start spawns Command at cols x rows and begins reading its output.
// It's called from the first Paint, not Reconcile, deliberately: Paint
// is the first point a real size is known. Starting the pty (and its
// vt.Screen) at that size directly, rather than at some placeholder
// size resized later, matters because there's no way to guarantee
// Paint runs before the child's first output arrives — a vt.Screen
// created too small (e.g. 1x1) can lose content to scrolling before a
// later Resize ever gets a chance to preserve it, since Resize only
// copies over the overlapping region of the old and new buffers (see
// vt.Screen.Resize).
func (w *terminalWidget) start(cols, rows int) {
	if w.opts.Command == nil {
		return
	}
	w.cmd = w.opts.Command
	p, err := pty.Start(w.cmd)
	if err != nil {
		w.mu.Lock()
		w.exited, w.exitErr = true, err
		w.mu.Unlock()
		return
	}
	_ = p.Resize(term.Size{Cols: cols, Rows: rows})
	w.pty = p
	w.parser = vt.NewParser()
	w.screen = vt.NewScreen(cols, rows)
	w.lastCols, w.lastRows = cols, rows
	go w.readLoop()
}

func (w *terminalWidget) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := w.pty.Read(buf)
		if n > 0 {
			w.mu.Lock()
			w.parser.Feed(buf[:n], w.screen)
			resp := w.screen.TakeResponses()
			w.mu.Unlock()
			if len(resp) > 0 {
				_, _ = w.pty.Write(resp)
			}
		}
		if err != nil {
			waitErr := w.cmd.Wait()
			w.mu.Lock()
			w.exited, w.exitErr = true, waitErr
			w.mu.Unlock()
			return
		}
	}
}

func (w *terminalWidget) Paint(p *cell.Painter) {
	width, height := p.Size()
	if width <= 0 || height <= 0 {
		return
	}

	if !w.mounted {
		w.mounted = true
		w.start(width, height)
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.screen == nil {
		if w.exited && w.exitErr != nil {
			p.Text(0, 0, "[failed to start: "+w.exitErr.Error()+"]", cell.Style{Attr: cell.AttrBold})
		}
		return
	}

	if width != w.lastCols || height != w.lastRows {
		w.screen.Resize(width, height)
		if w.pty != nil {
			_ = w.pty.Resize(term.Size{Cols: width, Rows: height})
		}
		w.lastCols, w.lastRows = width, height
	}

	src := w.screen.Buffer()
	for y := range height {
		for x := range width {
			p.SetRawCell(x, y, src.At(x, y))
		}
	}

	if w.focused && !w.exited {
		if cx, cy, visible := w.screen.Cursor(); visible && cx < width && cy < height {
			c := src.At(cx, cy)
			c.Style.Attr |= cell.AttrReverse
			p.SetRawCell(cx, cy, c)
		}
	}

	if w.exited {
		msg := "[exited]"
		if w.exitErr != nil {
			msg = "[exited: " + w.exitErr.Error() + "]"
		}
		p.Text(0, height-1, msg, cell.Style{Attr: cell.AttrBold | cell.AttrReverse})
	}
}

func (w *terminalWidget) HandleEvent(e input.Event) tui.Cmd {
	w.mu.Lock()
	exited, exitErr := w.exited, w.exitErr
	w.mu.Unlock()

	if exited {
		if w.opts.OnExit == nil {
			return nil
		}
		if msg := w.opts.OnExit(exitErr); msg != nil {
			return func() tui.Msg { return msg }
		}
		return nil
	}

	if w.pty == nil {
		return nil
	}
	if b := encodeEvent(e); len(b) > 0 {
		_, _ = w.pty.Write(b)
	}
	return nil
}

func (w *terminalWidget) Focusable() bool         { return true }
func (w *terminalWidget) SetFocused(focused bool) { w.focused = focused }

// WantsRawTab and ReleaseKey implement tui.RawKeyClaimer — see
// TerminalOptions.WantsRawTab/ReleaseKey. Tab itself, once claimed,
// needs no special handling here: encodeEvent already encodes KeyTab
// like any other key.
func (w *terminalWidget) WantsRawTab() bool { return w.opts.WantsRawTab }
func (w *terminalWidget) ReleaseKey() input.KeyEvent {
	if w.opts.ReleaseKey != (input.KeyEvent{}) {
		return w.opts.ReleaseKey
	}
	return input.KeyEvent{Rune: '\\', Mod: input.ModCtrl}
}

// Close closes the pty master, which reliably delivers SIGHUP to the
// child (standard pty semantics) so the readLoop's own cmd.Wait()
// reaps it — the same "closing the master is enough" empirical finding
// from M3's pty package tests, not something Terminal needs to
// duplicate by also signaling the process itself.
func (w *terminalWidget) Close() error {
	if w.pty == nil {
		return nil
	}
	return w.pty.Close()
}
