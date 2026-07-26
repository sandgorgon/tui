package widget

import (
	"strings"
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/layout"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

type modalHostModel struct {
	open   bool
	events []tui.Msg
}

func (m *modalHostModel) Init() tui.Cmd { return nil }
func (m *modalHostModel) Update(msg tui.Msg) (tui.Model, tui.Cmd) {
	m.events = append(m.events, msg)
	return m, nil
}
func (m *modalHostModel) View() tui.Node {
	return tui.Box(layout.Horizontal,
		tui.Child(layout.Fill(1), tui.Focusable("bg", tui.Text("background", cell.Style{}), func(e input.Event) tui.Msg { return "bg" })),
		tui.Child(layout.Length(0), Modal(
			Field1AndField2(),
			ModalOptions{Theme: style.DefaultDark(), Open: m.open, Title: "Confirm", Width: 20, Height: 8},
		)),
	)
}

// Field1AndField2 returns a small modal body with two focusable
// fields, for tests that need to Tab between them. Each gets
// Length(3): tui.Focusable always reserves a 1-cell border on every
// side, so Length(1) would give its wrapped Text a zero-height
// painter and render nothing.
func Field1AndField2() tui.Node {
	return tui.Box(layout.Vertical,
		tui.Child(layout.Length(3), tui.Focusable("f1", tui.Text("field1", cell.Style{}), func(e input.Event) tui.Msg { return "f1" })),
		tui.Child(layout.Length(3), tui.Focusable("f2", tui.Text("field2", cell.Style{}), func(e input.Event) tui.Msg { return "f2" })),
	)
}

func TestModalNotPaintedWhenClosed(t *testing.T) {
	m := &modalHostModel{open: false}
	app := tui.NewApp(m, 30, 10)
	if strings.Contains(app.Buffer().String(), "Confirm") {
		t.Error("closed Modal should not appear in the frame")
	}
}

func TestModalPaintedWhenOpen(t *testing.T) {
	m := &modalHostModel{open: true}
	app := tui.NewApp(m, 30, 10)
	got := app.Buffer().String()
	if !strings.Contains(got, "Confirm") {
		t.Errorf("Buffer = %q, want the open modal's title", got)
	}
	if !strings.Contains(got, "field1") || !strings.Contains(got, "field2") {
		t.Errorf("Buffer = %q, want the modal body's fields", got)
	}
}

func TestModalClaimsFocusExclusivelyWhileOpen(t *testing.T) {
	m := &modalHostModel{open: true}
	app := tui.NewApp(m, 30, 10)

	cmds := app.HandleInput(input.KeyEvent{Rune: 'x'})
	if len(cmds) != 1 || cmds[0]() != "f1" {
		t.Fatalf("expected the modal's first field to be focused while open, got cmds=%v", cmds)
	}

	app.HandleInput(input.KeyEvent{Key: input.KeyTab})
	cmds = app.HandleInput(input.KeyEvent{Rune: 'y'})
	if len(cmds) != 1 || cmds[0]() != "f2" {
		t.Fatalf("expected Tab to move focus to the modal's second field, got cmds=%v", cmds)
	}

	// Background must never receive focus while the modal is open, no
	// matter how many times Tab cycles.
	app.HandleInput(input.KeyEvent{Key: input.KeyTab})
	cmds = app.HandleInput(input.KeyEvent{Rune: 'z'})
	if len(cmds) != 1 || cmds[0]() != "f1" {
		t.Fatalf("expected Tab to cycle back to the modal's first field (not the background), got cmds=%v", cmds)
	}
}

func TestModalClosingReturnsFocusToBackground(t *testing.T) {
	m := &modalHostModel{open: true}
	app := tui.NewApp(m, 30, 10)
	// sanity: modal has focus while open
	if cmds := app.HandleInput(input.KeyEvent{Rune: 'x'}); len(cmds) != 1 || cmds[0]() != "f1" {
		t.Fatalf("setup: expected modal field focused, got %v", cmds)
	}

	m.open = false
	app.Dispatch("noop-to-force-rerender") // widgetHostModel-style Update ignores unknown msgs; modalHostModel just records them, that's fine

	cmds := app.HandleInput(input.KeyEvent{Rune: 'x'})
	if len(cmds) != 1 || cmds[0]() != "bg" {
		t.Fatalf("expected background focused after modal closed, got %v", cmds)
	}
}

func TestModalBodyStatePersistsAcrossCloseReopenWhenNodeAlwaysPresent(t *testing.T) {
	// The modal Node is always in the tree (Open just toggles), so its
	// body's retained widgets are never disposed — this test exists to
	// document that choice, not to re-verify disposal itself (see
	// tui/dispose_test.go for that).
	m := &modalHostModel{open: true}
	app := tui.NewApp(m, 30, 10)
	m.open = false
	app.Dispatch("noop")
	m.open = true
	app.Dispatch("noop")

	if !strings.Contains(app.Buffer().String(), "field1") {
		t.Error("expected modal body to still render correctly after close/reopen")
	}
}
