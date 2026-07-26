package widget

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/tui"
)

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
	// the cell there should have AttrReverse toggled on to represent
	// it (see Terminal.Paint).
	got := buf.At(0, 0).Style.Attr & cell.AttrReverse
	if got == 0 {
		t.Error("expected the cursor cell to have AttrReverse set while focused")
	}

	if err := tr.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}
