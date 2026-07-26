package widget

import (
	"strings"
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

func TestTabsPaintShowsAllLabels(t *testing.T) {
	node := Tabs([]string{"one", "two", "three"}, 1, style.DefaultDark(), nil)
	buf := cell.NewBuffer(20, 1)
	paintNode(t, node, buf)

	got := buf.String()
	for _, label := range []string{"one", "two", "three"} {
		if !strings.Contains(got, label) {
			t.Errorf("Buffer %q missing label %q", got, label)
		}
	}
}

func TestTabsForwardsEventsThroughOnEvent(t *testing.T) {
	var got input.Event
	node := Tabs([]string{"a", "b"}, 0, style.DefaultDark(), func(e input.Event) tui.Msg {
		got = e
		return "next-tab"
	})

	m := &widgetHostModel{node: node}
	app := tui.NewApp(m, 20, 1)
	cmds := app.HandleInput(input.KeyEvent{Key: input.KeyRight})
	if len(cmds) != 1 || cmds[0]() != "next-tab" {
		t.Fatalf("expected onEvent's Msg via HandleInput's Cmd, got %v", cmds)
	}
	if got != (input.KeyEvent{Key: input.KeyRight}) {
		t.Errorf("onEvent received %v", got)
	}
}
