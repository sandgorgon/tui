package widget

import (
	"strings"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

// TreeRow is one visible row of a Tree, already flattened by the
// caller from whatever hierarchical data it owns — respecting which
// nodes are currently expanded is the caller's job (the tree's shape
// is business state, per docs/DESIGN.md §3.1, exactly like List's
// items), not something Tree itself understands. Building this list
// is typically a short recursive walk over the caller's own node type,
// skipping the children of any node that isn't Expanded.
type TreeRow struct {
	Label       string
	Depth       int // nesting depth, for indentation
	HasChildren bool
	Expanded    bool
	// Loading is true while a lazy-load Cmd for this node's children is
	// in flight — see Tree's doc comment.
	Loading bool
}

// Tree is a focusable, vertically-scrolling, expand/collapse tree
// view — a thin variant of List's exact scrolling/cursor machinery
// (see List's doc comment) operating on a caller-flattened []TreeRow
// instead of a []string, since knowing which rows are currently
// visible (i.e. which ancestors are expanded) requires walking the
// caller's own hierarchical data, which Tree has no way to do itself.
// Lazy-loading a node's children is handled entirely by the
// application: onEvent typically reacts to Enter/Right on a
// HasChildren-but-not-yet-loaded row by returning a Msg that kicks off
// a Cmd to fetch and merge in that node's children — the ordinary
// Cmd/Msg pattern (docs/DESIGN.md §3.1) any other async work uses.
// Tree itself has no concept of "loading" beyond rendering
// TreeRow.Loading as an indicator.
func Tree(rows []TreeRow, cursor int, theme style.Theme, onEvent func(input.Event) tui.Msg) tui.Node {
	return tui.Component(nil, treeProps{
		rows: rows, cursor: cursor, theme: theme, onEvent: onEvent,
	}, func() tui.Widget {
		return &treeWidget{}
	})
}

type treeProps struct {
	rows    []TreeRow
	cursor  int
	theme   style.Theme
	onEvent func(input.Event) tui.Msg
}

type treeWidget struct {
	treeProps
	focused      bool
	scrollOffset int
}

func (w *treeWidget) Reconcile(props any) bool {
	w.treeProps = props.(treeProps)
	return true
}

func (w *treeWidget) Paint(p *cell.Painter) {
	width, height := p.Size()
	if width <= 0 || height <= 0 || len(w.rows) == 0 {
		return
	}
	w.scrollOffset = clampScroll(w.scrollOffset, w.cursor, len(w.rows), height)

	base := w.theme.Text()
	selected := cell.Style{Fg: base.Fg, Bg: base.Bg, Attr: base.Attr | cell.AttrReverse}

	for row := range height {
		idx := w.scrollOffset + row
		if idx >= len(w.rows) {
			break
		}
		r := w.rows[idx]

		rowStyle := base
		marker := "  "
		if idx == w.cursor {
			rowStyle = selected
			marker = ". "
			if w.focused {
				marker = "> "
			}
		}

		expand := "  "
		switch {
		case r.Loading:
			expand = "· "
		case r.HasChildren && r.Expanded:
			expand = "▾ "
		case r.HasChildren:
			expand = "▸ "
		}

		text := marker + strings.Repeat("  ", r.Depth) + expand + r.Label
		p.Text(0, row, text, rowStyle)
	}
}

func (w *treeWidget) HandleEvent(e input.Event) tui.Cmd {
	if w.onEvent == nil {
		return nil
	}
	msg := w.onEvent(e)
	if msg == nil {
		return nil
	}
	return func() tui.Msg { return msg }
}

func (w *treeWidget) Focusable() bool         { return true }
func (w *treeWidget) SetFocused(focused bool) { w.focused = focused }
