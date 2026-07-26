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

	if w.onEvent == nil {
		return nil
	}
	msg := w.onEvent(e)
	if msg == nil {
		return nil
	}
	return func() tui.Msg { return msg }
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
