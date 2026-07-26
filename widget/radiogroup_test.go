package widget

import (
	"strings"
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

func TestRadioGroupPaintMarksSelected(t *testing.T) {
	node := RadioGroup([]string{"small", "medium", "large"}, 1, style.DefaultDark(), nil)
	buf := cell.NewBuffer(12, 3)
	paintNode(t, node, buf)

	rows := strings.Split(buf.String(), "\n")
	if !strings.HasPrefix(rows[0], "( )") {
		t.Errorf("row 0 = %q, want unselected marker", rows[0])
	}
	if !strings.HasPrefix(rows[1], "(•)") {
		t.Errorf("row 1 = %q, want selected marker", rows[1])
	}
	if !strings.HasPrefix(rows[2], "( )") {
		t.Errorf("row 2 = %q, want unselected marker", rows[2])
	}
}

func TestRadioGroupForwardsEventsThroughOnEvent(t *testing.T) {
	var got input.Event
	node := RadioGroup([]string{"a", "b"}, 0, style.DefaultDark(), func(e input.Event) tui.Msg {
		got = e
		return "next"
	})

	m := &widgetHostModel{node: node}
	app := tui.NewApp(m, 12, 2)
	cmds := app.HandleInput(input.KeyEvent{Key: input.KeyDown})
	if len(cmds) != 1 || cmds[0]() != "next" {
		t.Fatalf("expected onEvent's Msg via HandleInput's Cmd, got %v", cmds)
	}
	if got != (input.KeyEvent{Key: input.KeyDown}) {
		t.Errorf("onEvent received %v", got)
	}
}
