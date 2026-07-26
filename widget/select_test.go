package widget

import (
	"strings"
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

func TestSelectClosedShowsOnlySelectedLabel(t *testing.T) {
	node := Select([]string{"red", "green", "blue"}, 1, 1, SelectOptions{Theme: style.DefaultDark(), Open: false}, nil)
	buf := cell.NewBuffer(10, 5)
	paintNode(t, node, buf)

	got := buf.String()
	if !strings.Contains(got, "▾ green") {
		t.Errorf("Buffer = %q, want closed control showing \"green\"", got)
	}
	if strings.Contains(got, "red") || strings.Contains(got, "blue") {
		t.Errorf("Buffer = %q, want other options hidden while closed", got)
	}
}

func TestSelectOpenShowsOptionListWithSelectionMarker(t *testing.T) {
	node := Select([]string{"red", "green", "blue"}, 1, 1, SelectOptions{Theme: style.DefaultDark(), Open: true}, nil)
	// border(1) + arrow line(1) + 3 options(3) + border(1) = 6 rows;
	// width 12 leaves room for "marker(2) + check(2) + green(5)" = 9
	// inner columns without truncating.
	buf := cell.NewBuffer(12, 6)
	paintNode(t, node, buf)

	rows := strings.Split(buf.String(), "\n")
	if !strings.Contains(rows[1], "▴ green") {
		t.Errorf("row 1 = %q, want open control (flipped arrow) showing \"green\"", rows[1])
	}
	if !strings.Contains(rows[2], "red") {
		t.Errorf("row 2 = %q, want \"red\"", rows[2])
	}
	if !strings.Contains(rows[3], "• green") {
		t.Errorf("row 3 = %q, want the selection marker \"•\" next to \"green\" (cursor is also on green here)", rows[3])
	}
	if !strings.Contains(rows[4], "blue") {
		t.Errorf("row 4 = %q, want \"blue\"", rows[4])
	}
}

func TestSelectCursorAndSelectionMarkersAreIndependent(t *testing.T) {
	// cursor on "blue" (idx 2) but selected is still "red" (idx 0).
	node := Select([]string{"red", "green", "blue"}, 0, 2, SelectOptions{Theme: style.DefaultDark(), Open: true}, nil)
	buf := cell.NewBuffer(12, 6)
	paintNode(t, node, buf)

	rows := strings.Split(buf.String(), "\n")
	if !strings.Contains(rows[2], "• red") {
		t.Errorf("row 2 = %q, want the selection marker on \"red\"", rows[2])
	}
	if !strings.Contains(rows[4], "blue") || strings.ContainsRune(rows[4], '•') {
		t.Errorf("row 4 = %q, want the cursor on \"blue\" with no selection marker (\"blue\" isn't selected)", rows[4])
	}
}

func TestSelectForwardsEventsThroughOnEvent(t *testing.T) {
	var got input.Event
	node := Select([]string{"a", "b"}, 0, 0, SelectOptions{Theme: style.DefaultDark(), Open: true}, func(e input.Event) tui.Msg {
		got = e
		return "chosen"
	})

	m := &widgetHostModel{node: node}
	app := tui.NewApp(m, 10, 5)
	cmds := app.HandleInput(input.KeyEvent{Key: input.KeyEnter})
	if len(cmds) != 1 || cmds[0]() != "chosen" {
		t.Fatalf("expected onEvent's Msg via HandleInput's Cmd, got %v", cmds)
	}
	if got != (input.KeyEvent{Key: input.KeyEnter}) {
		t.Errorf("onEvent received %v", got)
	}
}

func TestSelectRetainsScrollOffsetAcrossReconcile(t *testing.T) {
	options := make([]string, 20)
	for i := range options {
		options[i] = string(rune('a' + i))
	}

	var tr tui.Tree
	tr.Reconcile(Select(options, 19, 19, SelectOptions{Theme: style.DefaultDark(), Open: true}, nil))
	buf := cell.NewBuffer(10, 6)
	tr.Paint(cell.NewPainter(buf))
	first := buf.String()

	fresh := append([]string(nil), options...)
	tr.Reconcile(Select(fresh, 19, 19, SelectOptions{Theme: style.DefaultDark(), Open: true}, nil))
	buf2 := cell.NewBuffer(10, 6)
	tr.Paint(cell.NewPainter(buf2))

	if got := buf2.String(); got != first {
		t.Errorf("second paint =\n%q\nwant (unchanged)\n%q", got, first)
	}
}
