package widget

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/layout"
	"github.com/sandgorgon/tui/tui"
)

// terminalAndFocusable builds a two-widget frame — a Terminal and a
// second Focusable widget forwarding a tagged Msg — for tests that
// need to check whether Tab/a release key moved focus away from the
// Terminal.
func terminalAndFocusable(opts TerminalOptions) *tui.App {
	m := &widgetHostModel{node: tui.Box(layout.Horizontal,
		tui.Child(layout.Fill(1), Terminal(opts)),
		tui.Child(layout.Fill(1), tui.Focusable("other", tui.Text("other", cell.Style{}), func(e input.Event) tui.Msg { return "other" })),
	)}
	return tui.NewApp(m, 30, 6)
}

// waitFor polls check every 20ms until it returns true or timeout
// elapses, calling paint before each check (Terminal's output arrives
// asynchronously on a background goroutine — see its doc comment on
// why nothing pushes a repaint automatically).
func waitFor(t *testing.T, timeout time.Duration, paint func(), check func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		paint()
		if check() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("condition not met within %s", timeout)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestTerminalShowsCommandOutput(t *testing.T) {
	node := Terminal(TerminalOptions{Command: exec.Command("printf", "hello-terminal")})
	buf := cell.NewBuffer(30, 3)
	var tr tui.Tree
	tr.Reconcile(node)

	waitFor(t, 2*time.Second, func() { tr.Paint(cell.NewPainter(buf)) }, func() bool {
		return strings.Contains(buf.String(), "hello-terminal")
	})

	if err := tr.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestTerminalHandleEventWritesToChild(t *testing.T) {
	node := Terminal(TerminalOptions{Command: exec.Command("cat")})
	buf := cell.NewBuffer(30, 3)
	var tr tui.Tree
	tr.Reconcile(node)
	tr.Paint(cell.NewPainter(buf)) // establishes the pty's size before writing

	widget := tr.Focusables()[0]
	widget.HandleEvent(input.KeyEvent{Rune: 'z'})

	waitFor(t, 2*time.Second, func() { tr.Paint(cell.NewPainter(buf)) }, func() bool {
		return strings.ContainsRune(buf.String(), 'z')
	})

	if err := tr.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// TestTerminalMouseEventsForwardLocalCoordinates confirms that once a
// child has opted into mouse reporting (DECSET 1000/1006 — see
// TestTerminalMouseEventsSuppressedWithoutMouseMode for the off-by-
// default case this guards), Terminal needs no widget-level change for
// App's mouse hit-testing to work: it forwards the input.Event straight
// into encodeMouse, and App has already translated a click's
// coordinates to be local to Terminal's own bounds by the time
// HandleEvent sees it (see tui.App.hitTest) — exactly what a real
// program running inside (vim, tmux, ...) expects: mouse coordinates
// relative to its own pane, not the outer screen.
//
// The child shell first enables mouse mode itself, then echoes a
// "READY" sentinel before exec-ing into "cat -v" — the test waits for
// that sentinel so the click isn't sent until vt.Screen has actually
// processed the DECSET sequence (Terminal gates mouse forwarding on
// vt.Screen.MouseMode(), which updates asynchronously as pty output is
// read). "cat -v" (not plain "cat") renders control bytes as visible
// caret notation (ESC becomes "^[") instead of our own vt.Parser
// interpreting the echoed escape sequence as a real mouse report,
// which — since it's a valid CSI sequence — wouldn't produce any
// visible text to assert against at all.
func TestTerminalMouseEventsForwardLocalCoordinates(t *testing.T) {
	m := &widgetHostModel{node: tui.Box(layout.Horizontal,
		tui.Child(layout.Length(5), tui.Text("spacer", cell.Style{})),
		tui.Child(layout.Fill(1), Terminal(TerminalOptions{
			Command: exec.Command("sh", "-c", `printf '\033[?1000h\033[?1006h'; echo READY; exec cat -v`),
		})),
	)}
	app := tui.NewApp(m, 30, 6)
	defer closeApp(t, app)

	waitFor(t, 2*time.Second, func() { app.Dispatch("noop") }, func() bool {
		return strings.Contains(app.Buffer().String(), "READY")
	})

	// Absolute (7,1): the Terminal pane starts at absolute X=5, so this
	// should reach it as local (2,1) — encoded as SGR X=3,Y=2 (1-based).
	app.HandleInput(input.MouseEvent{X: 7, Y: 1, Button: input.MouseLeft})

	waitFor(t, 2*time.Second, func() { app.Dispatch("noop") }, func() bool {
		return strings.Contains(app.Buffer().String(), "^[[<0;3;2M")
	})
}

// TestTerminalMouseEventsSuppressedWithoutMouseMode is the regression
// test for the bug fixed alongside TestTerminalMouseEventsForwardLocalCoordinates's
// update: a child that never enables mouse reporting (a plain shell, as
// opposed to vim/tmux/etc.) must not receive raw SGR mouse bytes as
// literal keyboard input. Before the fix, Terminal forwarded every
// mouse event unconditionally regardless of the child's own DECSET
// state; now it checks vt.Screen.MouseMode() first.
func TestTerminalMouseEventsSuppressedWithoutMouseMode(t *testing.T) {
	m := &widgetHostModel{node: tui.Box(layout.Horizontal,
		tui.Child(layout.Length(5), tui.Text("spacer", cell.Style{})),
		tui.Child(layout.Fill(1), Terminal(TerminalOptions{Command: exec.Command("cat", "-v")})),
	)}
	app := tui.NewApp(m, 30, 6)
	defer closeApp(t, app)

	// Give the child a moment to start before the click, then a moment
	// after for a (bug-triggering) write to have shown up if it were
	// going to.
	waitFor(t, 2*time.Second, func() { app.Dispatch("noop") }, func() bool { return true })
	app.HandleInput(input.MouseEvent{X: 7, Y: 1, Button: input.MouseLeft})
	time.Sleep(100 * time.Millisecond)
	app.Dispatch("noop")

	if strings.Contains(app.Buffer().String(), "^[[<") {
		t.Errorf("Buffer = %q, want no forwarded mouse sequence (child never enabled mouse mode)", app.Buffer().String())
	}
}

func TestTerminalOnExitFiresFromHandleEvent(t *testing.T) {
	var exitErr error
	var exitSeen bool
	node := Terminal(TerminalOptions{
		Command: exec.Command("true"),
		OnExit: func(err error) tui.Msg {
			exitSeen, exitErr = true, err
			return "exited"
		},
	})
	buf := cell.NewBuffer(10, 2)
	var tr tui.Tree
	tr.Reconcile(node)

	widget := tr.Focusables()[0]
	waitFor(t, 2*time.Second, func() { tr.Paint(cell.NewPainter(buf)) }, func() bool {
		return strings.Contains(buf.String(), "[exited]")
	})

	cmd := widget.HandleEvent(input.KeyEvent{Rune: 'x'})
	if cmd == nil || cmd() != "exited" {
		t.Fatalf("expected OnExit's Msg via HandleEvent's Cmd, got %v", cmd)
	}
	if !exitSeen {
		t.Fatal("OnExit was not called")
	}
	if exitErr != nil {
		t.Errorf("exitErr = %v, want nil (\"true\" exits cleanly)", exitErr)
	}

	if err := tr.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestTerminalFailedStartShowsError(t *testing.T) {
	node := Terminal(TerminalOptions{Command: exec.Command("/nonexistent-binary-xyz-123")})
	buf := cell.NewBuffer(40, 3)
	paintNode(t, node, buf)

	if !strings.Contains(buf.String(), "failed to start") {
		t.Errorf("Buffer = %q, want a failed-to-start message", buf.String())
	}
}

func TestTerminalNoCommandPaintsNothing(t *testing.T) {
	node := Terminal(TerminalOptions{})
	buf := cell.NewBuffer(10, 3)
	paintNode(t, node, buf)

	if strings.TrimSpace(buf.String()) != "" {
		t.Errorf("Buffer = %q, want blank (no Command given)", buf.String())
	}
}

func TestTerminalCloseIsIdempotentAndSafeWithoutStart(t *testing.T) {
	var tr tui.Tree
	tr.Reconcile(Terminal(TerminalOptions{}))
	if err := tr.Close(); err != nil {
		t.Errorf("Close with no Command started: %v", err)
	}
}

func TestTerminalCursorShownWhenFocused(t *testing.T) {
	node := Terminal(TerminalOptions{Command: exec.Command("cat")})
	buf := cell.NewBuffer(10, 3)
	var tr tui.Tree
	tr.Reconcile(node)
	tr.Paint(cell.NewPainter(buf))

	widget := tr.Focusables()[0]
	widget.SetFocused(true)
	tr.Paint(cell.NewPainter(buf))

	// A fresh cat process's screen starts with the cursor at (0,0);
	// the cell there should have AttrReverse forced on to represent
	// it (see Terminal.Paint).
	got := buf.At(0, 0).Style.Attr & cell.AttrReverse
	if got == 0 {
		t.Error("expected the cursor cell to have AttrReverse set while focused")
	}

	if err := tr.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// TestTerminalCursorStaysVisibleOverReverseVideoContent guards against
// a real bug: Paint used to XOR AttrReverse onto the cursor cell, which
// canceled the attribute back off (making the cursor invisible, blending
// into the surrounding reverse-video text) whenever the child program
// had already drawn that cell in reverse video itself — e.g. vim visual
// mode, less's search highlighting, or a status line. Forcing the bit on
// (OR, not XOR) fixes it.
func TestTerminalCursorStaysVisibleOverReverseVideoContent(t *testing.T) {
	node := Terminal(TerminalOptions{Command: exec.Command("sh", "-c", "printf '\\033[7m \\033[0m\\033[H'; cat")})
	buf := cell.NewBuffer(10, 3)
	var tr tui.Tree
	tr.Reconcile(node)

	// Wait for the child's reverse-video cell (and cursor-home) to land
	// before focusing, so the assertion below isn't racing startup.
	waitFor(t, 2*time.Second, func() { tr.Paint(cell.NewPainter(buf)) }, func() bool {
		return buf.At(0, 0).Style.Attr&cell.AttrReverse != 0
	})

	widget := tr.Focusables()[0]
	widget.SetFocused(true)
	tr.Paint(cell.NewPainter(buf))

	got := buf.At(0, 0).Style.Attr & cell.AttrReverse
	if got == 0 {
		t.Error("cursor cell lost AttrReverse over already-reverse-video content — cursor is invisible")
	}

	if err := tr.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestTerminalWithoutWantsRawTabStillNavigatesOnTab(t *testing.T) {
	app := terminalAndFocusable(TerminalOptions{Command: exec.Command("cat")})
	defer closeApp(t, app)

	cmds := app.HandleInput(input.KeyEvent{Key: input.KeyTab})
	if len(cmds) != 0 {
		t.Fatalf("unexpected cmd from Tab: %v", cmds)
	}
	cmds = app.HandleInput(input.KeyEvent{Rune: 'x'})
	if len(cmds) != 1 || cmds[0]() != "other" {
		t.Fatalf("expected Tab to have moved focus to \"other\" (WantsRawTab is false by default), got cmds=%v", cmds)
	}
}

func TestTerminalWantsRawTabKeepsFocusOnTab(t *testing.T) {
	app := terminalAndFocusable(TerminalOptions{Command: exec.Command("cat"), WantsRawTab: true})
	defer closeApp(t, app)

	app.HandleInput(input.KeyEvent{Key: input.KeyTab})
	cmds := app.HandleInput(input.KeyEvent{Rune: 'x'})
	if len(cmds) != 0 {
		t.Fatalf("expected focus to remain on the Terminal after Tab (WantsRawTab is true), got cmds=%v", cmds)
	}
}

func TestTerminalReleaseKeyDefaultsToCtrlBackslash(t *testing.T) {
	app := terminalAndFocusable(TerminalOptions{Command: exec.Command("cat"), WantsRawTab: true})
	defer closeApp(t, app)

	if cmds := app.HandleInput(input.KeyEvent{Rune: '\\', Mod: input.ModCtrl}); len(cmds) != 0 {
		t.Fatalf("release key itself should produce no Cmd, got %v", cmds)
	}
	cmds := app.HandleInput(input.KeyEvent{Rune: 'x'})
	if len(cmds) != 1 || cmds[0]() != "other" {
		t.Fatalf("expected Ctrl+\\ to release focus to \"other\", got cmds=%v", cmds)
	}
}

func TestTerminalCustomReleaseKey(t *testing.T) {
	app := terminalAndFocusable(TerminalOptions{
		Command: exec.Command("cat"), WantsRawTab: true,
		ReleaseKey: input.KeyEvent{Key: input.KeyF12},
	})
	defer closeApp(t, app)

	// The default (Ctrl+\) must NOT release focus for this instance.
	app.HandleInput(input.KeyEvent{Rune: '\\', Mod: input.ModCtrl})
	if cmds := app.HandleInput(input.KeyEvent{Rune: 'x'}); len(cmds) != 0 {
		t.Fatalf("expected Ctrl+\\ to have no special effect here, got cmds=%v", cmds)
	}

	app.HandleInput(input.KeyEvent{Key: input.KeyF12})
	cmds := app.HandleInput(input.KeyEvent{Rune: 'x'})
	if len(cmds) != 1 || cmds[0]() != "other" {
		t.Fatalf("expected the custom release key (F12) to move focus, got cmds=%v", cmds)
	}
}

// closeApp closes app entirely — used by the raw-tab tests
// above to release the Terminal's pty/goroutine, since
// terminalAndFocusable doesn't expose the App's retained tree
// directly the way the other tests' tui.Tree do.
func closeApp(t *testing.T, app *tui.App) {
	t.Helper()
	if err := app.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}
