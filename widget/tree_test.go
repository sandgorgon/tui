package widget

import (
	"strings"
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

func TestTreePaintShowsDepthAndExpandMarkers(t *testing.T) {
	rows := []TreeRow{
		{Label: "root", Depth: 0, HasChildren: true, Expanded: true},
		{Label: "child", Depth: 1, HasChildren: false},
		{Label: "lazy", Depth: 1, HasChildren: true, Expanded: false},
	}
	node := Tree(rows, 0, style.DefaultDark(), nil)
	buf := cell.NewBuffer(20, 3)
	paintNode(t, node, buf)

	got := strings.Split(buf.String(), "\n")
	if !strings.Contains(got[0], "▾ root") {
		t.Errorf("row 0 = %q, want expanded marker + \"root\"", got[0])
	}
	if !strings.Contains(got[1], "  child") {
		t.Errorf("row 1 = %q, want indented \"child\" with no expand marker", got[1])
	}
	if !strings.Contains(got[2], "▸ lazy") {
		t.Errorf("row 2 = %q, want collapsed marker + \"lazy\"", got[2])
	}
}

func TestTreePaintShowsLoadingIndicator(t *testing.T) {
	rows := []TreeRow{{Label: "fetching", Depth: 0, HasChildren: true, Loading: true}}
	node := Tree(rows, 0, style.DefaultDark(), nil)
	buf := cell.NewBuffer(20, 1)
	paintNode(t, node, buf)

	if !strings.Contains(buf.String(), "· fetching") {
		t.Errorf("Buffer = %q, want loading marker", buf.String())
	}
}

func TestTreeForwardsEventsThroughOnEvent(t *testing.T) {
	var got input.Event
	rows := []TreeRow{{Label: "a"}, {Label: "b"}}
	node := Tree(rows, 0, style.DefaultDark(), func(e input.Event) tui.Msg {
		got = e
		return "expand"
	})

	m := &widgetHostModel{node: node}
	app := tui.NewApp(m, 20, 3)
	cmds := app.HandleInput(input.KeyEvent{Key: input.KeyRight})
	if len(cmds) != 1 || cmds[0]() != "expand" {
		t.Fatalf("expected onEvent's Msg via HandleInput's Cmd, got %v", cmds)
	}
	if got != (input.KeyEvent{Key: input.KeyRight}) {
		t.Errorf("onEvent received %v", got)
	}
}

func TestTreeRetainsScrollOffsetAcrossReconcile(t *testing.T) {
	rows := make([]TreeRow, 20)
	for i := range rows {
		rows[i] = TreeRow{Label: string(rune('a' + i))}
	}

	var tr tui.Tree
	tr.Reconcile(Tree(rows, 19, style.DefaultDark(), nil))
	buf := cell.NewBuffer(10, 4)
	tr.Paint(cell.NewPainter(buf))
	first := buf.String()

	fresh := append([]TreeRow(nil), rows...)
	tr.Reconcile(Tree(fresh, 19, style.DefaultDark(), nil))
	buf2 := cell.NewBuffer(10, 4)
	tr.Paint(cell.NewPainter(buf2))

	if got := buf2.String(); got != first {
		t.Errorf("second paint =\n%q\nwant (unchanged)\n%q", got, first)
	}
}
