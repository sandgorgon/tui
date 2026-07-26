package widget

import (
	"strings"
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

func TestCheckboxGroupPaintShowsIndependentChecks(t *testing.T) {
	node := CheckboxGroup([]string{"a", "b", "c"}, []bool{true, false, true}, 1, style.DefaultDark(), nil)
	buf := cell.NewBuffer(10, 3)
	paintNode(t, node, buf)

	rows := strings.Split(buf.String(), "\n")
	if !strings.HasPrefix(rows[0], "[x] a") {
		t.Errorf("row 0 = %q, want checked", rows[0])
	}
	if !strings.HasPrefix(rows[1], "[ ] b") {
		t.Errorf("row 1 = %q, want unchecked", rows[1])
	}
	if !strings.HasPrefix(rows[2], "[x] c") {
		t.Errorf("row 2 = %q, want checked", rows[2])
	}
}

func TestCheckboxGroupChecksIndependentOfCursor(t *testing.T) {
	// cursor is on row 0, but only row 2 is checked — the two must not
	// be conflated (see the widget's doc comment on why they're
	// separate props).
	node := CheckboxGroup([]string{"a", "b", "c"}, []bool{false, false, true}, 0, style.DefaultDark(), nil)
	buf := cell.NewBuffer(10, 3)
	paintNode(t, node, buf)

	rows := strings.Split(buf.String(), "\n")
	if !strings.HasPrefix(rows[0], "[ ] a") {
		t.Errorf("row 0 = %q, want unchecked despite being under the cursor", rows[0])
	}
	if !strings.HasPrefix(rows[2], "[x] c") {
		t.Errorf("row 2 = %q, want checked despite not being under the cursor", rows[2])
	}
}

func TestCheckboxGroupForwardsEventsThroughOnEvent(t *testing.T) {
	var got input.Event
	node := CheckboxGroup([]string{"a", "b"}, []bool{false, false}, 0, style.DefaultDark(), func(e input.Event) tui.Msg {
		got = e
		return "toggled"
	})

	m := &widgetHostModel{node: node}
	app := tui.NewApp(m, 10, 2)
	cmds := app.HandleInput(input.KeyEvent{Rune: ' '})
	if len(cmds) != 1 || cmds[0]() != "toggled" {
		t.Fatalf("expected onEvent's Msg via HandleInput's Cmd, got %v", cmds)
	}
	if got != (input.KeyEvent{Rune: ' '}) {
		t.Errorf("onEvent received %v", got)
	}
}

func TestCheckboxGroupClickForwardsOptionIndexAsIs(t *testing.T) {
	var got input.Event
	node := CheckboxGroup([]string{"a", "b", "c"}, []bool{false, false, false}, 0, style.DefaultDark(), func(e input.Event) tui.Msg {
		got = e
		return "clicked"
	})
	m := &widgetHostModel{node: node}
	app := tui.NewApp(m, 10, 3)

	cmds := app.HandleInput(input.MouseEvent{X: 1, Y: 2, Button: input.MouseLeft})
	if len(cmds) != 1 || cmds[0]() != "clicked" {
		t.Fatalf("expected onEvent's Msg from the click, got cmds=%v", cmds)
	}
	me, ok := got.(input.MouseEvent)
	if !ok || me.Y != 2 {
		t.Errorf("onEvent received %v, want Y=2 (\"c\"'s index; no border to offset by)", got)
	}
}

func TestCheckboxGroupClickPastLastOptionProducesNoEvent(t *testing.T) {
	called := false
	node := CheckboxGroup([]string{"only"}, []bool{false}, 0, style.DefaultDark(), func(e input.Event) tui.Msg {
		called = true
		return "x"
	})
	m := &widgetHostModel{node: node}
	app := tui.NewApp(m, 10, 3)

	app.HandleInput(input.MouseEvent{X: 1, Y: 2, Button: input.MouseLeft})
	if called {
		t.Error("clicking past the last option should not forward to onEvent")
	}
}
