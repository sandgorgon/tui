package widget

import (
	"strings"
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/layout"
	"github.com/sandgorgon/tui/tui"
)

func linesNode(lines []string) tui.Node {
	children := make([]tui.BoxChild, len(lines))
	for i, l := range lines {
		children[i] = tui.Child(layout.Length(1), tui.Text(l, cell.Style{}))
	}
	return tui.Box(layout.Vertical, children...)
}

func TestViewportShowsTopOfContentInitially(t *testing.T) {
	content := linesNode([]string{"one", "two", "three", "four", "five"})
	node := Viewport(content, 5)

	buf := cell.NewBuffer(6, 2)
	paintNode(t, node, buf)

	want := "one   \ntwo   "
	if got := buf.String(); got != want {
		t.Errorf("Buffer = %q, want %q", got, want)
	}
}

func TestViewportScrollsWithArrowKeys(t *testing.T) {
	content := linesNode([]string{"one", "two", "three", "four", "five"})
	m := &widgetHostModel{node: Viewport(content, 5)}
	app := tui.NewApp(m, 6, 2)

	app.HandleInput(input.KeyEvent{Key: input.KeyDown})
	app.HandleInput(input.KeyEvent{Key: input.KeyDown})

	want := "three \nfour  "
	if got := app.Buffer().String(); got != want {
		t.Errorf("Buffer after two Down = %q, want %q", got, want)
	}
}

func TestViewportPgDownAndEndClampToContent(t *testing.T) {
	content := linesNode([]string{"1", "2", "3", "4", "5", "6", "7", "8"})
	m := &widgetHostModel{node: Viewport(content, 8)}
	app := tui.NewApp(m, 3, 3)

	app.HandleInput(input.KeyEvent{Key: input.KeyEnd})
	want := "6  \n7  \n8  "
	if got := app.Buffer().String(); got != want {
		t.Errorf("Buffer after End = %q, want %q (clamped so the last row is visible)", got, want)
	}

	app.HandleInput(input.KeyEvent{Key: input.KeyHome})
	want = "1  \n2  \n3  "
	if got := app.Buffer().String(); got != want {
		t.Errorf("Buffer after Home = %q, want %q", got, want)
	}
}

func TestViewportRetainsScrollAcrossReconcile(t *testing.T) {
	var tr tui.Tree
	content := linesNode([]string{"a", "b", "c", "d"})
	tr.Reconcile(Viewport(content, 4))

	buf := cell.NewBuffer(3, 2)
	tr.Paint(cell.NewPainter(buf))
	if !strings.HasPrefix(buf.String(), "a") {
		t.Fatalf("initial paint = %q, want to start with \"a\"", buf.String())
	}
}
