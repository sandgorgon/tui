package widget

import (
	"strings"
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

func testColumns() []Column {
	return []Column{{Title: "Name", Width: 6}, {Title: "Age", Width: 4}}
}

func TestTablePaintShowsHeaderAndRows(t *testing.T) {
	rows := [][]string{{"alice", "30"}, {"bob", "25"}}
	node := Table(testColumns(), rows, 0, TableOptions{Theme: style.DefaultDark(), SortColumn: -1}, nil)
	buf := cell.NewBuffer(20, 3)
	paintNode(t, node, buf)

	got := strings.Split(buf.String(), "\n")
	if !strings.HasPrefix(got[0], "Name") || !strings.Contains(got[0], "Age") {
		t.Errorf("header row = %q", got[0])
	}
	if !strings.HasPrefix(got[1], "alice") {
		t.Errorf("row 1 = %q, want to start with \"alice\"", got[1])
	}
	if !strings.HasPrefix(got[2], "bob") {
		t.Errorf("row 2 = %q, want to start with \"bob\"", got[2])
	}
}

func TestTableSortIndicatorShownOnSortColumn(t *testing.T) {
	// "Age" needs to be wide enough to fit its title *and* the " ▼"
	// indicator (5 cells) — testColumns()'s width-4 Age column is
	// intentionally too narrow for that, exercised separately by
	// TestTableSortIndicatorTruncatedByNarrowColumn below.
	columns := []Column{{Title: "Name", Width: 6}, {Title: "Age", Width: 6}}
	node := Table(columns, nil, 0, TableOptions{Theme: style.DefaultDark(), SortColumn: 1, SortDescending: true}, nil)
	buf := cell.NewBuffer(20, 1)
	paintNode(t, node, buf)

	got := buf.String()
	if !strings.Contains(got, "Age ▼") {
		t.Errorf("header = %q, want descending sort arrow next to \"Age\"", got)
	}
	if strings.Contains(got, "Name ▲") || strings.Contains(got, "Name ▼") {
		t.Errorf("header = %q, want no sort arrow on \"Name\"", got)
	}
}

func TestTableSortIndicatorTruncatedByNarrowColumn(t *testing.T) {
	// testColumns()'s "Age" column is width 4 — too narrow to fit
	// "Age ▼" (5 cells). padTruncate enforces the column width
	// contract unconditionally, so the indicator gets cut off along
	// with the rest of any overlong header text; that's expected, not
	// a bug (see TestTableSortIndicatorShownOnSortColumn for the
	// column-wide-enough case).
	node := Table(testColumns(), nil, 0, TableOptions{Theme: style.DefaultDark(), SortColumn: 1, SortDescending: true}, nil)
	buf := cell.NewBuffer(20, 1)
	paintNode(t, node, buf)

	got := strings.Split(buf.String(), "\n")[0]
	if !strings.HasPrefix(got, "Name   Age ") {
		t.Errorf("header = %q, want the sort arrow truncated away by the narrow column", got)
	}
	if strings.ContainsRune(got, '▼') {
		t.Errorf("header = %q, the sort arrow should have been truncated away entirely", got)
	}
}

func TestTableColumnResizeWithShiftArrows(t *testing.T) {
	rows := [][]string{{"a", "1"}}
	m := &widgetHostModel{node: Table(testColumns(), rows, 0, TableOptions{Theme: style.DefaultDark(), SortColumn: -1}, nil)}
	app := tui.NewApp(m, 20, 2)

	before := app.Buffer().String()
	app.HandleInput(input.KeyEvent{Key: input.KeyRight, Mod: input.ModShift}) // widen column 0 ("Name")
	after := app.Buffer().String()

	if before == after {
		t.Fatal("expected header/row layout to change after widening column 0")
	}
	rows2 := strings.Split(after, "\n")
	if !strings.HasPrefix(rows2[0], "Name  ") { // one extra space from the widened column
		t.Errorf("header after resize = %q, want a wider \"Name\" column", rows2[0])
	}
}

func TestTableColumnResizeHasMinimumWidth(t *testing.T) {
	m := &widgetHostModel{node: Table([]Column{{Title: "X", Width: tableMinColumnWidth}}, nil, 0, TableOptions{Theme: style.DefaultDark(), SortColumn: -1}, nil)}
	app := tui.NewApp(m, 10, 2)

	for range 10 {
		app.HandleInput(input.KeyEvent{Key: input.KeyLeft, Mod: input.ModShift})
	}
	got := strings.Split(app.Buffer().String(), "\n")[0]
	if !strings.HasPrefix(got, "X  ") { // 3 cells: min width
		t.Errorf("header = %q, want column clamped to the minimum width", got)
	}
}

func TestTableLeftRightMovesCursorColumnAndHighlightsHeader(t *testing.T) {
	m := &widgetHostModel{node: Table(testColumns(), nil, 0, TableOptions{Theme: style.DefaultDark(), SortColumn: -1}, nil)}
	app := tui.NewApp(m, 20, 2)

	app.HandleInput(input.KeyEvent{Key: input.KeyRight})
	if got := app.Buffer().At(0, 0).Style.Underline; got != cell.UnderlineNone {
		t.Errorf("\"Name\" header underline = %v, want none after moving cursorCol off it", got)
	}
	if got := app.Buffer().At(7, 0).Style.Underline; got != cell.UnderlineSingle {
		t.Errorf("\"Age\" header underline = %v, want underlined after Right moved cursorCol onto it", got)
	}
}

func TestTableVirtualizesRows(t *testing.T) {
	rows := make([][]string, 50)
	for i := range rows {
		rows[i] = []string{string(rune('a' + i%26)), "0"}
	}
	node := Table(testColumns(), rows, 49, TableOptions{Theme: style.DefaultDark(), SortColumn: -1}, nil)
	buf := cell.NewBuffer(20, 5) // header + 4 visible rows, out of 50
	paintNode(t, node, buf)

	got := buf.String()
	if !strings.Contains(got, string(rune('a'+49%26))) {
		t.Errorf("Buffer should scroll to keep the selected (last) row visible:\n%s", got)
	}
}

func TestTableForwardsNonNavigationEventsThroughOnEvent(t *testing.T) {
	var got input.Event
	node := Table(testColumns(), nil, 0, TableOptions{Theme: style.DefaultDark(), SortColumn: -1}, func(e input.Event) tui.Msg {
		got = e
		return "activated"
	})

	m := &widgetHostModel{node: node}
	app := tui.NewApp(m, 20, 3)
	cmds := app.HandleInput(input.KeyEvent{Key: input.KeyEnter})
	if len(cmds) != 1 || cmds[0]() != "activated" {
		t.Fatalf("expected onEvent's Msg via HandleInput's Cmd, got %v", cmds)
	}
	if got != (input.KeyEvent{Key: input.KeyEnter}) {
		t.Errorf("onEvent received %v", got)
	}
}

func TestTableClickHeaderTranslatesToColumnWithSentinelRow(t *testing.T) {
	var got input.Event
	node := Table(testColumns(), nil, 0, TableOptions{Theme: style.DefaultDark(), SortColumn: -1}, func(e input.Event) tui.Msg {
		got = e
		return "header-clicked"
	})
	m := &widgetHostModel{node: node}
	app := tui.NewApp(m, 20, 3)

	// "Age" is columns[1], starting at X=7 (Name is 6 wide + 1 gap).
	cmds := app.HandleInput(input.MouseEvent{X: 8, Y: 0, Button: input.MouseLeft})
	if len(cmds) != 1 || cmds[0]() != "header-clicked" {
		t.Fatalf("expected onEvent's Msg from the header click, got cmds=%v", cmds)
	}
	me, ok := got.(input.MouseEvent)
	if !ok || me.X != 1 || me.Y != -1 {
		t.Errorf("onEvent received %v, want X=1 (Age column), Y=-1 (header sentinel)", got)
	}
}

func TestTableClickBodyRowTranslatesToRowAndColumn(t *testing.T) {
	var got input.Event
	rows := [][]string{{"alice", "30"}, {"bob", "25"}}
	node := Table(testColumns(), rows, 0, TableOptions{Theme: style.DefaultDark(), SortColumn: -1}, func(e input.Event) tui.Msg {
		got = e
		return "row-clicked"
	})
	m := &widgetHostModel{node: node}
	app := tui.NewApp(m, 20, 3)

	// Row 2 on screen (Y=2) is the second data row ("bob"); X=0 is Name.
	cmds := app.HandleInput(input.MouseEvent{X: 0, Y: 2, Button: input.MouseLeft})
	if len(cmds) != 1 || cmds[0]() != "row-clicked" {
		t.Fatalf("expected onEvent's Msg from the row click, got cmds=%v", cmds)
	}
	me, ok := got.(input.MouseEvent)
	if !ok || me.X != 0 || me.Y != 1 {
		t.Errorf("onEvent received %v, want X=0 (Name column), Y=1 (\"bob\"'s row index)", got)
	}
}

func TestTableClickInColumnGapProducesNoEvent(t *testing.T) {
	called := false
	node := Table(testColumns(), [][]string{{"a", "1"}}, 0, TableOptions{Theme: style.DefaultDark(), SortColumn: -1}, func(e input.Event) tui.Msg {
		called = true
		return "x"
	})
	m := &widgetHostModel{node: node}
	app := tui.NewApp(m, 20, 3)

	app.HandleInput(input.MouseEvent{X: 6, Y: 0, Button: input.MouseLeft}) // the 1-cell gap after "Name"
	if called {
		t.Error("clicking the gap between columns should not forward to onEvent")
	}
}

func TestTableClickPastLastRowProducesNoEvent(t *testing.T) {
	called := false
	node := Table(testColumns(), [][]string{{"a", "1"}}, 0, TableOptions{Theme: style.DefaultDark(), SortColumn: -1}, func(e input.Event) tui.Msg {
		called = true
		return "x"
	})
	m := &widgetHostModel{node: node}
	app := tui.NewApp(m, 20, 5) // room for 4 body rows, only 1 row of data

	app.HandleInput(input.MouseEvent{X: 0, Y: 3, Button: input.MouseLeft})
	if called {
		t.Error("clicking past the last row should not forward to onEvent")
	}
}

func TestTableDragResizesColumnBoundary(t *testing.T) {
	rows := [][]string{{"a", "1"}}
	m := &widgetHostModel{node: Table(testColumns(), rows, 0, TableOptions{Theme: style.DefaultDark(), SortColumn: -1}, nil)}
	app := tui.NewApp(m, 20, 2)

	// The gap after "Name" (width 6) is at X=6. Press there, then drag
	// 3 cells right, widening "Name" by 3.
	app.HandleInput(input.MouseEvent{X: 6, Y: 0, Button: input.MouseLeft})
	app.HandleInput(input.MouseEvent{X: 9, Y: 0, Button: input.MouseLeft, Drag: true})

	got := strings.Split(app.Buffer().String(), "\n")[0]
	if !strings.HasPrefix(got, "Name      Age") { // 6+3=9-wide "Name", plus its 1-cell gap
		t.Errorf("header after drag = %q, want \"Name\" widened by 3 columns", got)
	}
}

func TestTableDragResizeClampsToMinimumWidth(t *testing.T) {
	rows := [][]string{{"a", "1"}}
	m := &widgetHostModel{node: Table(testColumns(), rows, 0, TableOptions{Theme: style.DefaultDark(), SortColumn: -1}, nil)}
	app := tui.NewApp(m, 20, 2)

	app.HandleInput(input.MouseEvent{X: 6, Y: 0, Button: input.MouseLeft})
	app.HandleInput(input.MouseEvent{X: -10, Y: 0, Button: input.MouseLeft, Drag: true})

	got := strings.Split(app.Buffer().String(), "\n")[0]
	if !strings.HasPrefix(got, "Nam ") { // clamped to tableMinColumnWidth (3)
		t.Errorf("header after drag past minimum = %q, want \"Name\" clamped to width 3", got)
	}
}

func TestTableDragResizeEndsOnRelease(t *testing.T) {
	rows := [][]string{{"a", "1"}}
	m := &widgetHostModel{node: Table(testColumns(), rows, 0, TableOptions{Theme: style.DefaultDark(), SortColumn: -1}, nil)}
	app := tui.NewApp(m, 20, 2)

	app.HandleInput(input.MouseEvent{X: 6, Y: 0, Button: input.MouseLeft})
	app.HandleInput(input.MouseEvent{X: 9, Y: 0, Button: input.MouseLeft, Drag: true})
	app.HandleInput(input.MouseEvent{X: 9, Y: 0, Button: input.MouseRelease})
	// A further move-with-button-down event after release must not
	// resume the old drag.
	app.HandleInput(input.MouseEvent{X: 15, Y: 0, Button: input.MouseLeft, Drag: true})

	got := strings.Split(app.Buffer().String(), "\n")[0]
	if !strings.HasPrefix(got, "Name      Age") {
		t.Errorf("header after release = %q, want the width from before release to stick, not keep growing", got)
	}
}

func TestTableDragResizeAbandonedWhenRowChangesMidDrag(t *testing.T) {
	rows := [][]string{{"a", "1"}, {"b", "2"}}
	m := &widgetHostModel{node: Table(testColumns(), rows, 0, TableOptions{Theme: style.DefaultDark(), SortColumn: -1}, nil)}
	app := tui.NewApp(m, 20, 3)

	app.HandleInput(input.MouseEvent{X: 6, Y: 0, Button: input.MouseLeft})
	// A continuation event landing on a different row — e.g. because
	// the drag left Table's tracked bounds and App delivered it with
	// raw, untranslated coordinates (see handleColumnDrag's doc
	// comment) — must not be treated as part of the same drag.
	app.HandleInput(input.MouseEvent{X: 9, Y: 1, Button: input.MouseLeft, Drag: true})

	got := strings.Split(app.Buffer().String(), "\n")[0]
	if !strings.HasPrefix(got, "Name   Age") { // unchanged width (6-wide "Name" + its gap)
		t.Errorf("header = %q, want the drag abandoned (no width change) once the row changed", got)
	}
}

func TestTableClickInColumnGapWithoutDragStillProducesNoEvent(t *testing.T) {
	called := false
	node := Table(testColumns(), [][]string{{"a", "1"}}, 0, TableOptions{Theme: style.DefaultDark(), SortColumn: -1}, func(e input.Event) tui.Msg {
		called = true
		return "x"
	})
	m := &widgetHostModel{node: node}
	app := tui.NewApp(m, 20, 3)

	app.HandleInput(input.MouseEvent{X: 6, Y: 0, Button: input.MouseLeft})
	app.HandleInput(input.MouseEvent{X: 6, Y: 0, Button: input.MouseRelease})
	if called {
		t.Error("a plain click (press+release, no drag) on the gap should still not forward to onEvent")
	}
}

func TestTableRetainsColumnWidthsAcrossReconcile(t *testing.T) {
	var tr tui.Tree
	buf := cell.NewBuffer(20, 2)

	build := func() tui.Node {
		return Table(testColumns(), nil, 0, TableOptions{Theme: style.DefaultDark(), SortColumn: -1}, nil)
	}
	tr.Reconcile(build())
	tr.Paint(cell.NewPainter(buf))

	widget := tr.Focusables()[0]
	widget.SetFocused(true) // HandleEvent doesn't require focus itself, but keep it realistic
	widget.HandleEvent(input.KeyEvent{Key: input.KeyRight, Mod: input.ModShift})

	// Reconcile a *brand new* Node built the same way Model.View()
	// would every frame — not just repaint the same one — to prove the
	// resize survived reconciliation, not just repeated painting.
	tr.Reconcile(build())
	buf2 := cell.NewBuffer(20, 2)
	tr.Paint(cell.NewPainter(buf2))

	if got := buf2.String(); !strings.HasPrefix(got, "Name  ") {
		t.Errorf("header after reconcile = %q, want the widened \"Name\" column to have survived", got)
	}
}
