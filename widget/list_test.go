package widget

import (
	"strings"
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

func TestListPaintShowsCursorAndBorder(t *testing.T) {
	node := List([]string{"a", "b", "c"}, 1, ListOptions{Theme: style.DefaultDark()}, nil)
	buf := cell.NewBuffer(10, 5)
	paintNode(t, node, buf)

	rows := strings.Split(buf.String(), "\n")
	if !strings.HasPrefix(rows[0], "┌") {
		t.Fatalf("row 0 = %q, want a top border", rows[0])
	}
	if !strings.Contains(rows[2], ". b") { // cursor is on "b" (index 1), unfocused marker
		t.Errorf("row for cursor item = %q, want it to contain \". b\"", rows[2])
	}
}

func TestListMultiSelectRendersCheckboxes(t *testing.T) {
	node := List([]string{"a", "b"}, 0, ListOptions{
		Theme:    style.DefaultDark(),
		Selected: []bool{false, true},
	}, nil)
	buf := cell.NewBuffer(12, 4)
	paintNode(t, node, buf)

	rows := strings.Split(buf.String(), "\n")
	if !strings.Contains(rows[1], "[ ] a") {
		t.Errorf("row 1 = %q, want unchecked box for item a", rows[1])
	}
	if !strings.Contains(rows[2], "[x] b") {
		t.Errorf("row 2 = %q, want checked box for item b", rows[2])
	}
}

func TestListRetainsScrollOffsetAcrossReconcile(t *testing.T) {
	items := make([]string, 20)
	for i := range items {
		items[i] = strings.Repeat("x", 1) + string(rune('a'+i))
	}

	var tr tui.Tree
	tr.Reconcile(List(items, 19, ListOptions{Theme: style.DefaultDark()}, nil))
	buf := cell.NewBuffer(10, 6) // 4 visible rows inside the border
	tr.Paint(cell.NewPainter(buf))
	first := buf.String()

	// A fresh items slice (as Model.View() would build every frame)
	// with the same cursor must render identically — proving the
	// retained scrollOffset survived, not reset to 0.
	fresh := append([]string(nil), items...)
	tr.Reconcile(List(fresh, 19, ListOptions{Theme: style.DefaultDark()}, nil))
	buf2 := cell.NewBuffer(10, 6)
	tr.Paint(cell.NewPainter(buf2))

	if got := buf2.String(); got != first {
		t.Errorf("second paint =\n%q\nwant (unchanged)\n%q", got, first)
	}
	if !strings.Contains(first, "t") { // just sanity that some scrolled-to content rendered
		t.Fatalf("expected scrolled content to include the last items, got:\n%s", first)
	}
}

func TestListForwardsEventsThroughOnEvent(t *testing.T) {
	var got input.Event
	node := List([]string{"a"}, 0, ListOptions{Theme: style.DefaultDark()}, func(e input.Event) tui.Msg {
		got = e
		return "moved"
	})

	m := &widgetHostModel{node: node}
	app := tui.NewApp(m, 10, 5)
	cmds := app.HandleInput(input.KeyEvent{Key: input.KeyDown})
	if len(cmds) != 1 || cmds[0]() != "moved" {
		t.Fatalf("expected onEvent's Msg to come back via HandleInput's Cmd, got %v", cmds)
	}
	if got != (input.KeyEvent{Key: input.KeyDown}) {
		t.Errorf("onEvent received %v", got)
	}
}

// widgetHostModel is a minimal tui.Model that always shows a single
// widget Node, for tests that need a real App (e.g. to exercise focus,
// which requires App's focus tracking — Tree alone doesn't have it).
type widgetHostModel struct {
	node tui.Node
}

func (m *widgetHostModel) Init() tui.Cmd                       { return nil }
func (m *widgetHostModel) Update(tui.Msg) (tui.Model, tui.Cmd) { return m, nil }
func (m *widgetHostModel) View() tui.Node                      { return m.node }

func TestListInsideAppShowsFocusMarker(t *testing.T) {
	m := &widgetHostModel{node: List([]string{"a", "b"}, 0, ListOptions{Theme: style.DefaultDark()}, nil)}
	app := tui.NewApp(m, 10, 5)

	rows := strings.Split(app.Buffer().String(), "\n")
	if !strings.Contains(rows[1], "> a") {
		t.Errorf("focused list row = %q, want \"> a\" marker (List is the only focusable widget, so it starts focused)", rows[1])
	}
}
