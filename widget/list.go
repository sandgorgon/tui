package widget

import (
	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

// ListOptions configures List.
type ListOptions struct {
	Theme style.Theme

	// Selected turns on multi-select rendering when non-nil: item i
	// draws with a "[x]"/"[ ]" checkbox reflecting Selected[i] (false
	// for any index past len(Selected)), independent of which row the
	// cursor is on. Which items are selected is business state — the
	// app almost always needs to know it, e.g. to act on "delete the
	// selected items" — so, like Cursor, it's a prop the caller owns
	// and supplies fresh every frame rather than state retained inside
	// the widget; see List's own doc comment for the one thing that is
	// retained.
	Selected []bool
}

// List is a focusable, vertically-scrolling, optionally multi-select
// list of text rows, styled from a style.Theme and drawing its own
// focus border directly (no tui.Focusable wrapper needed — compare
// tui.List, the minimal M8 stand-in this supersedes). Which row is
// under the cursor is the caller's business state, passed in fresh
// every frame via cursor (docs/DESIGN.md §3.1); the widget's own
// retained instance owns only scrollOffset, the ephemeral state needed
// to keep the cursor row visible as the list scrolls. onEvent receives
// every input event while List is focused and, like tui.Focusable,
// whatever Msg it returns is fed back into the App's event loop.
func List(items []string, cursor int, opts ListOptions, onEvent func(input.Event) tui.Msg) tui.Node {
	return tui.Component(nil, listProps{
		items: items, cursor: cursor, opts: opts, onEvent: onEvent,
	}, func() tui.Widget {
		return &listWidget{}
	})
}

type listProps struct {
	items   []string
	cursor  int
	opts    ListOptions
	onEvent func(input.Event) tui.Msg
}

type listWidget struct {
	listProps
	focused      bool
	scrollOffset int
}

func (w *listWidget) Reconcile(props any) bool {
	w.listProps = props.(listProps)
	return true
}

func (w *listWidget) Paint(p *cell.Painter) {
	width, height := p.Size()
	if width < 2 || height < 2 {
		return
	}

	border := w.opts.Theme.BorderStyle()
	if w.focused {
		border = w.opts.Theme.FocusStyle()
	}
	drawBorder(p, width, height, border)

	inner := p.Clip(cell.Rect{X: 1, Y: 1, W: width - 2, H: height - 2})
	_, innerH := inner.Size()
	if innerH <= 0 || len(w.items) == 0 {
		return
	}
	w.scrollOffset = clampScroll(w.scrollOffset, w.cursor, len(w.items), innerH)

	base := w.opts.Theme.Text()
	selected := cell.Style{Fg: base.Fg, Bg: base.Bg, Attr: base.Attr | cell.AttrReverse}

	for row := range innerH {
		idx := w.scrollOffset + row
		if idx >= len(w.items) {
			break
		}

		rowStyle := base
		marker := "  "
		if idx == w.cursor {
			rowStyle = selected
			marker = ". "
			if w.focused {
				marker = "> "
			}
		}

		text := marker + w.items[idx]
		if w.opts.Selected != nil {
			box := "[ ] "
			if idx < len(w.opts.Selected) && w.opts.Selected[idx] {
				box = "[x] "
			}
			text = marker + box + w.items[idx]
		}
		inner.Text(0, row, text, rowStyle)
	}
}

func (w *listWidget) HandleEvent(e input.Event) tui.Cmd {
	if me, ok := e.(input.MouseEvent); ok {
		idx, ok := w.itemAt(me.Y)
		if !ok {
			return nil // clicked the border or past the last item — no target
		}
		translated := me
		translated.Y = idx // Y becomes the clicked item's index, not a pixel row
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

// itemAt translates a MouseEvent's Y — local to List's full painted
// bounds, border included, per App's hit-testing (see tui.App.hitTest)
// — into an item index, or ok=false if it lands on the border or past
// the last item.
func (w *listWidget) itemAt(y int) (idx int, ok bool) {
	row := y - 1 // top border
	if row < 0 {
		return 0, false
	}
	idx = w.scrollOffset + row
	if idx < 0 || idx >= len(w.items) {
		return 0, false
	}
	return idx, true
}

func (w *listWidget) Focusable() bool         { return true }
func (w *listWidget) SetFocused(focused bool) { w.focused = focused }

// clampScroll adjusts offset by the minimum amount needed to keep
// cursor within [offset, offset+visible), then keeps it within
// [0, max(itemCount-visible, 0)] — shared by List and Viewport.
func clampScroll(offset, cursor, itemCount, visible int) int {
	switch {
	case cursor < offset:
		offset = cursor
	case cursor >= offset+visible:
		offset = cursor - visible + 1
	}
	if max := itemCount - visible; offset > max {
		offset = max
	}
	if offset < 0 {
		offset = 0
	}
	return offset
}
