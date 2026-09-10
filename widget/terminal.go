package widget

import (
	"fmt"
	"os/exec"
	"sync"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/pty"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/term"
	"github.com/sandgorgon/tui/tui"
	"github.com/sandgorgon/tui/vt"
)

// wheelScrollLines is how many lines a single mouse-wheel tick scrolls
// scrollback by — matches the common terminal-emulator default.
const wheelScrollLines = 3

// TerminalOptions configures Terminal.
type TerminalOptions struct {
	// Command starts a fresh pty-attached child the first time this
	// Node mounts — read once, like TextInput's Value: a Terminal owns
	// its child process for its whole retained lifetime, not something
	// restarted or resynced from props on every frame.
	Command *exec.Cmd

	// OnExit, if non-nil, is called after the child process exits
	// (err is nil on a clean exit), and its Msg is delivered on
	// whatever App.Dispatch call runs next (tui.PendingMsgSource) — see
	// Terminal's doc comment for why it can't fire the instant the
	// child actually exits.
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

	// Theme styles the "[scrollback N/M]" indicator Paint draws while
	// scrolled back (see DisableScrollback) — via Theme.ChromeText(),
	// since the indicator is chrome overlaid on the pty's own content,
	// not content itself. The zero Theme still renders it, just
	// without Border/Chrome coloring.
	Theme style.Theme

	// DisableScrollback, if true, turns off Terminal's own handling of
	// mouse-wheel and PageUp/PageDown scrolling through vt.Screen's
	// scrollback — for a host that wants those events forwarded to the
	// child (or handled) some other way instead. Off by default: wheel
	// ticks and plain PageUp/PageDown scroll scrollback whenever the
	// child hasn't claimed mouse reporting and isn't running a full-
	// screen alt-screen program (see vt.Screen.AltScreenActive).
	DisableScrollback bool
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
// vt.Screen's scrollback (up to 10,000 primary-screen lines scrolled
// off the top — see vt/scrollback.go) is otherwise unreachable data,
// so Terminal itself scrolls it into view: a wheel tick or PageUp/
// PageDown (unless DisableScrollback is set) scrolls back, Paint blends
// scrollback lines above the live buffer while scrolled and draws a
// "[scrollback N/M]" indicator, and new child output or any forwarded
// keystroke snaps the view back to live — see paintScrolled,
// scrollByLocked, and HandleEvent.
//
// A Terminal owns real OS resources — the pty master fd and a
// background goroutine reading it — for as long as its retained
// Widget instance exists; it implements io.Closer so those are
// released the moment its Node stops appearing in the tree (see
// tui/dispose.go).
//
// One known, deliberate limitation, a consequence of this library's
// change-driven redraw model (docs/DESIGN.md §8, the same tradeoff
// M6's ticker-based prototype existed specifically to avoid needing):
// the pty's output updates Terminal's internal vt.Screen state
// continuously in the background, but that only becomes visible the
// next time the App happens to render a frame for any reason — an app
// hosting a Terminal that wants live-updating output while otherwise
// idle needs to drive its own periodic redraw (e.g. a self-
// rescheduling Tick Cmd).
//
// OnExit is a sibling case of the same underlying gap — a retained
// Widget has no channel of its own into the App's Cmd/Msg loop (only
// Model.Update/HandleEvent can originate a Cmd), so there's no way to
// push a notification the instant a background goroutine notices the
// child died — but it doesn't cost an input event to work around:
// Terminal implements tui.PendingMsgSource, so OnExit's Msg is
// delivered on whatever App.Dispatch call happens to run next (a real
// input event's own raw Dispatch(Msg(e)), or the same periodic redraw
// Cmd already needed for live output above), not from HandleEvent. A
// keystroke arriving before the exit is noticed still reaches
// HandleEvent, which by then just discards it (the child is gone,
// there's nowhere to forward it) exactly as it always would have for
// a dead pty — it's no longer *spent proving* the child exited, so a
// widget the app switches to on OnExit isn't missing its first
// keystroke the way an event-triggered notification would (#33).
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

	mu           sync.Mutex
	parser       *vt.Parser
	screen       *vt.Screen
	exited       bool
	exitErr      error
	exitNotified bool

	lastCols, lastRows int
	focused            bool

	// scrollOffset is how many lines back from live the view is
	// scrolled (0 = live) — see scrollByLocked, HandleEvent, and
	// Paint's paintScrolled.
	scrollOffset int
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
			// New output snaps the view back to live, matching real
			// terminal scrollback behavior — a caller mid-review of
			// history isn't silently reading stale content while the
			// child keeps producing more of it underneath.
			w.scrollOffset = 0
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
	offset := w.scrollOffset
	if offset > 0 {
		w.paintScrolled(p, width, height, offset)
	} else {
		for y := range height {
			for x := range width {
				p.SetRawCell(x, y, src.At(x, y))
			}
		}
	}

	// The cursor's position is only meaningful in the live view — real
	// terminals hide it the same way while scrolled back.
	if w.focused && !w.exited && offset == 0 {
		if cx, cy, visible := w.screen.Cursor(); visible && cx < width && cy < height {
			c := src.At(cx, cy)
			c.Style.Attr |= cell.AttrReverse
			p.SetRawCell(cx, cy, c)
		}
	}

	if offset > 0 {
		w.paintScrollIndicator(p, width, offset)
	}

	if w.exited {
		msg := "[exited]"
		if w.exitErr != nil {
			msg = "[exited: " + w.exitErr.Error() + "]"
		}
		p.Text(0, height-1, msg, cell.Style{Attr: cell.AttrBold | cell.AttrReverse})
	}
}

// paintScrolled blends vt.Screen's scrollback above the live buffer's
// remaining rows for a scrollOffset of offset lines back from live:
// row y comes from scrollback when y < offset (oldest-to-newest, so
// the scrollback index climbs toward the live buffer as y does),
// otherwise from live row y-offset. Caller holds w.mu and has already
// checked offset > 0.
func (w *terminalWidget) paintScrolled(p *cell.Painter, width, height, offset int) {
	src := w.screen.Buffer()
	sbLen := w.screen.ScrollbackLen()
	for y := range height {
		if y >= offset {
			liveY := y - offset
			for x := range width {
				p.SetRawCell(x, y, src.At(x, liveY))
			}
			continue
		}
		line := w.screen.ScrollbackLine(sbLen - offset + y)
		for x := range width {
			if x < len(line) {
				p.SetRawCell(x, y, line[x])
			} else {
				p.SetRawCell(x, y, cell.Blank)
			}
		}
	}
}

// paintScrollIndicator draws a "[scrollback N/M]" marker in the top-
// right corner while scrolled back, styled via Theme.ChromeText() —
// chrome overlaid on the pty's own content, the same convention as a
// status bar painted on Border. Caller holds w.mu and has already
// checked offset > 0.
func (w *terminalWidget) paintScrollIndicator(p *cell.Painter, width, offset int) {
	msg := fmt.Sprintf("[scrollback %d/%d]", offset, w.screen.ScrollbackLen())
	if len(msg) > width {
		return
	}
	p.Text(width-len(msg), 0, msg, w.opts.Theme.ChromeText())
}

// scrollByLocked adjusts scrollOffset by delta lines — positive scrolls
// back into history, negative toward live — clamped to [0,
// ScrollbackLen()]. Caller holds w.mu.
func (w *terminalWidget) scrollByLocked(delta int) {
	off := max(w.scrollOffset+delta, 0)
	w.scrollOffset = min(off, w.screen.ScrollbackLen())
}

// TakePendingMsg implements tui.PendingMsgSource: once the child has
// exited, OnExit's Msg (if any) is reported exactly once, on whatever
// App.Dispatch call runs next — see Terminal's doc comment for why
// this, rather than HandleEvent, is what fires it.
func (w *terminalWidget) TakePendingMsg() tui.Msg {
	w.mu.Lock()
	exited, exitErr, notified := w.exited, w.exitErr, w.exitNotified
	if exited && !notified {
		w.exitNotified = true
	}
	w.mu.Unlock()

	if !exited || notified || w.opts.OnExit == nil {
		return nil
	}
	return w.opts.OnExit(exitErr)
}

func (w *terminalWidget) HandleEvent(e input.Event) tui.Cmd {
	w.mu.Lock()
	if w.exited || w.pty == nil {
		w.mu.Unlock()
		return nil
	}
	mouseEnabled := w.screen != nil && w.screen.MouseMode() != vt.MouseOff
	appCursorKeys := w.screen != nil && w.screen.AppCursorKeys()
	// Scrollback navigation only applies on the primary screen — a
	// full-screen alt-screen program (vim, htop, less, ...) manages its
	// own scrolling, and stealing PageUp/PageDown or wheel ticks from
	// it would fight rather than help (see vt.Screen.AltScreenActive).
	scrollbackActive := !w.opts.DisableScrollback && w.screen != nil && !w.screen.AltScreenActive()

	// A real terminal only forwards mouse events to the child once the
	// child has opted in via DECSET 1000/1002/1003 etc. (watched for by
	// vt.Screen, the same parser driving Paint) — otherwise a plain
	// shell with no mouse mode enabled receives raw SGR bytes as literal
	// keyboard input. When that's the case, a wheel tick scrolls
	// scrollback instead of being silently dropped.
	if me, isMouse := e.(input.MouseEvent); isMouse {
		if !mouseEnabled {
			if scrollbackActive {
				switch me.Button {
				case input.MouseWheelUp:
					w.scrollByLocked(wheelScrollLines)
				case input.MouseWheelDown:
					w.scrollByLocked(-wheelScrollLines)
				}
			}
			w.mu.Unlock()
			return nil
		}
	} else if ke, isKey := e.(input.KeyEvent); isKey && scrollbackActive && ke.Mod == 0 {
		switch ke.Key {
		case input.KeyPgUp:
			w.scrollByLocked(w.lastRows)
			w.mu.Unlock()
			return nil
		case input.KeyPgDown:
			w.scrollByLocked(-w.lastRows)
			w.mu.Unlock()
			return nil
		}
	}

	// Any event actually reaching the child snaps the view back to
	// live first, so a keystroke is never typed into a shell the user
	// can't currently see.
	w.scrollOffset = 0
	w.mu.Unlock()

	if b := encodeEvent(e, appCursorKeys); len(b) > 0 {
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
