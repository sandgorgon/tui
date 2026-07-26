package widget

import (
	"strings"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

// Column describes one column of a Table.
type Column struct {
	Title string
	Width int // initial width in cells; user-adjustable afterward, see Table's doc comment
}

// TableOptions configures Table.
type TableOptions struct {
	Theme style.Theme

	// SortColumn, if it indexes into columns, is shown in the header
	// with an up/down arrow (per SortDescending) marking it as the
	// current sort key. Table doesn't sort anything itself — see
	// Table's doc comment.
	SortColumn     int
	SortDescending bool
}

const tableMinColumnWidth = 3

// Table is a focusable, vertically-scrolling, virtualized grid (only
// currently-visible rows are painted, see Paint) with sortable,
// resizable columns. rows and their order are caller-owned business
// state: Table renders whatever order rows arrives in and only
// indicates opts.SortColumn/SortDescending in its header — it doesn't
// sort anything itself, exactly like List doesn't filter
// (docs/DESIGN.md §3.1). How the user picks a sort column is entirely
// up to the application's own onEvent handling (e.g. number keys) —
// Table has no notion of that.
//
// Column widths, by contrast, are the one thing Table does retain
// itself: they start from columns[i].Width at mount (like TextInput's
// Value) and are then user-adjustable — Left/Right picks a column,
// Shift+Left/Shift+Right resizes it — handled entirely inside
// HandleEvent with no onEvent round trip at all: pure display
// preference, not business state, the same reasoning as Viewport's
// scroll position.
func Table(columns []Column, rows [][]string, cursor int, opts TableOptions, onEvent func(input.Event) tui.Msg) tui.Node {
	return tui.Component(nil, tableProps{
		columns: columns, rows: rows, cursor: cursor, opts: opts, onEvent: onEvent,
	}, func() tui.Widget {
		return &tableWidget{}
	})
}

type tableProps struct {
	columns []Column
	rows    [][]string
	cursor  int
	opts    TableOptions
	onEvent func(input.Event) tui.Msg
}

type tableWidget struct {
	tableProps
	mounted   bool
	colWidths []int
	cursorCol int

	scrollOffset int
	focused      bool

	// draggingCol/dragRow/dragLastX track an in-progress column
	// boundary drag (see handleColumnDrag): draggingCol is -1 when
	// nothing is being dragged, otherwise the index of the column whose
	// right-edge boundary is being resized. dragRow is the local row
	// the drag started on; a continuation event landing on a different
	// row ends the drag defensively rather than risk mixing coordinate
	// spaces (see handleColumnDrag's doc comment).
	draggingCol int
	dragRow     int
	dragLastX   int
}

func (w *tableWidget) Reconcile(props any) bool {
	w.tableProps = props.(tableProps)

	switch {
	case !w.mounted:
		w.mounted = true
		w.colWidths = make([]int, len(w.columns))
		for i, c := range w.columns {
			w.colWidths[i] = max(c.Width, tableMinColumnWidth)
		}
		w.draggingCol = -1
	case len(w.colWidths) != len(w.columns):
		// Column count changed since mount (a genuinely different
		// table, not just new row data) — re-derive widths, keeping
		// whatever's still adjustable rather than resetting them all.
		widths := make([]int, len(w.columns))
		for i, c := range w.columns {
			if i < len(w.colWidths) {
				widths[i] = w.colWidths[i]
			} else {
				widths[i] = max(c.Width, tableMinColumnWidth)
			}
		}
		w.colWidths = widths
		// The column being dragged (if any) may no longer exist, or may
		// no longer mean what it did — safest to just abandon the drag.
		w.draggingCol = -1
	}
	if w.cursorCol >= len(w.columns) {
		w.cursorCol = max(len(w.columns)-1, 0)
	}
	return true
}

func (w *tableWidget) Paint(p *cell.Painter) {
	width, height := p.Size()
	if width <= 0 || height <= 0 || len(w.columns) == 0 {
		return
	}

	base := w.opts.Theme.Text()
	headerStyle := cell.Style{Fg: base.Fg, Bg: base.Bg, Attr: cell.AttrBold}

	col := 0
	for i, c := range w.columns {
		cw := w.colWidths[i]
		title := c.Title
		if i == w.opts.SortColumn {
			if w.opts.SortDescending {
				title += " ▼"
			} else {
				title += " ▲"
			}
		}
		hs := headerStyle
		if i == w.cursorCol && w.focused {
			hs.Underline = cell.UnderlineSingle
		}
		p.Text(col, 0, padTruncate(title, cw), hs)
		col += cw + 1 // 1-cell gap between columns
	}

	bodyHeight := height - 1
	if bodyHeight <= 0 || len(w.rows) == 0 {
		return
	}
	w.scrollOffset = clampScroll(w.scrollOffset, w.cursor, len(w.rows), bodyHeight)

	selected := cell.Style{Fg: base.Fg, Bg: base.Bg, Attr: base.Attr | cell.AttrReverse}
	for row := range bodyHeight {
		idx := w.scrollOffset + row
		if idx >= len(w.rows) {
			break
		}
		rowStyle := base
		if idx == w.cursor {
			rowStyle = selected
		}
		rowData := w.rows[idx]

		col := 0
		for i := range w.columns {
			cw := w.colWidths[i]
			text := ""
			if i < len(rowData) {
				text = rowData[i]
			}
			p.Text(col, row+1, padTruncate(text, cw), rowStyle)
			col += cw + 1
		}
	}
}

func (w *tableWidget) HandleEvent(e input.Event) tui.Cmd {
	if ke, ok := e.(input.KeyEvent); ok {
		switch {
		case ke.Key == input.KeyLeft && ke.Mod&input.ModShift != 0:
			if len(w.colWidths) > 0 {
				w.colWidths[w.cursorCol] = max(w.colWidths[w.cursorCol]-1, tableMinColumnWidth)
			}
			return nil
		case ke.Key == input.KeyRight && ke.Mod&input.ModShift != 0:
			if len(w.colWidths) > 0 {
				w.colWidths[w.cursorCol]++
			}
			return nil
		case ke.Key == input.KeyLeft:
			w.cursorCol = max(w.cursorCol-1, 0)
			return nil
		case ke.Key == input.KeyRight:
			w.cursorCol = min(w.cursorCol+1, max(len(w.columns)-1, 0))
			return nil
		}
	}

	if me, ok := e.(input.MouseEvent); ok {
		if w.handleColumnDrag(me) {
			return nil
		}
		col, ok := w.columnAt(me.X)
		if !ok {
			return nil // clicked a gap between columns
		}
		translated := me
		translated.X = col
		if me.Y == 0 {
			// The header row: Y=-1 is the sentinel for "this is a
			// header click, X is which column" — e.g. for toggling
			// sort by that column. Table doesn't sort anything itself,
			// same as everywhere else it defers to onEvent (see
			// Table's doc comment).
			translated.Y = -1
		} else {
			row, ok := w.rowAt(me.Y)
			if !ok {
				return nil
			}
			translated.Y = row
		}
		e = translated
	}

	if w.onEvent == nil {
		return nil
	}
	msg := w.onEvent(e)
	if msg == nil {
		return nil
	}
	return func() tui.Msg { return msg }
}

// columnAt translates an X coordinate local to Table's full painted
// bounds into a column index, or ok=false if it lands in the 1-cell
// gap between columns.
func (w *tableWidget) columnAt(x int) (col int, ok bool) {
	pos := 0
	for i, cw := range w.colWidths {
		if x >= pos && x < pos+cw {
			return i, true
		}
		pos += cw + 1 // 1-cell gap, see Paint
	}
	return 0, false
}

// boundaryAt reports whether x lands in the 1-cell gap immediately
// after column col (see Paint's col += cw + 1) — the drag handle for
// that column's right edge — or ok=false if x isn't in any such gap
// (there's no boundary after the last column).
func (w *tableWidget) boundaryAt(x int) (col int, ok bool) {
	pos := 0
	for i, cw := range w.colWidths {
		gapX := pos + cw
		if i < len(w.colWidths)-1 && x == gapX {
			return i, true
		}
		pos = gapX + 1
	}
	return 0, false
}

// handleColumnDrag implements click-and-drag column resizing: a press
// (Button set, Drag false) landing in the gap after a column (see
// boundaryAt) starts dragging that column's width; subsequent Drag
// events move colWidths[draggingCol] by the X delta since the last
// event; any MouseRelease ends it. It reports whether it consumed me
// (true) or me should fall through to normal column/row translation.
//
// A continuation event landing on a different row than the drag
// started on ends the drag rather than applying its delta — this is
// deliberately conservative: App.HandleInput only translates a
// MouseEvent's coordinates to be local to Table when the event's
// absolute position still falls inside Table's own tracked Rect (see
// tui.App.hitTest); if a drag gesture moves the mouse outside that
// Rect mid-drag, the event still gets delivered here (Table remains
// focused) but with raw, untranslated absolute coordinates, which
// would otherwise silently produce a garbage width jump when mixed
// with the local dragLastX from the previous event. A legitimately
// continued drag on the same row always reports the same local row
// each time, so a changed row is a reliable (if imperfect — see
// docs/DESIGN.md) signal that trust should end here.
func (w *tableWidget) handleColumnDrag(me input.MouseEvent) bool {
	if w.draggingCol >= 0 {
		switch {
		case me.Button == input.MouseRelease:
			w.draggingCol = -1
			return true
		case !me.Drag || me.Y != w.dragRow:
			w.draggingCol = -1
			return false
		default:
			dx := me.X - w.dragLastX
			w.colWidths[w.draggingCol] = max(w.colWidths[w.draggingCol]+dx, tableMinColumnWidth)
			w.dragLastX = me.X
			return true
		}
	}

	if me.Button == input.MouseLeft && !me.Drag {
		if col, ok := w.boundaryAt(me.X); ok {
			w.draggingCol = col
			w.dragRow = me.Y
			w.dragLastX = me.X
			return true
		}
	}
	return false
}

// rowAt translates a Y coordinate (already known to be > 0, i.e. below
// the header row) into a body row index, or ok=false past the last
// row.
func (w *tableWidget) rowAt(y int) (row int, ok bool) {
	idx := w.scrollOffset + (y - 1)
	if idx < 0 || idx >= len(w.rows) {
		return 0, false
	}
	return idx, true
}

func (w *tableWidget) Focusable() bool         { return true }
func (w *tableWidget) SetFocused(focused bool) { w.focused = focused }

// padTruncate returns s padded with trailing spaces to exactly width
// display columns, or hard-truncated to fit if it's already wider.
func padTruncate(s string, width int) string {
	w := stringWidth(s)
	if w > width {
		head, _, _ := breakToWidth(s, width)
		return head
	}
	return s + strings.Repeat(" ", width-w)
}
