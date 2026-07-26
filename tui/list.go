package tui

import (
	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
)

type listProps struct {
	items         []string
	selected      int
	style         cell.Style
	selectedStyle cell.Style
	onEvent       func(input.Event) Msg
}

// List is a minimal, focusable, vertically-scrolling list of text
// rows — a stand-in for the real List widget (M10), not its final API.
// Which row is selected is the caller's business state, passed in via
// selected on every frame (per docs/DESIGN.md §3.1, "which item is
// selected" is meaningful to the application, not cosmetic); List's
// own retained Widget instance owns only scrollOffset, the minimum
// needed to keep the selected row visible as the list scrolls — the
// kind of purely ephemeral state the retained-widget model exists for.
// Wrap it in Focusable to receive key events and participate in
// Tab/Shift-Tab traversal.
func List(items []string, selected int, style, selectedStyle cell.Style, onEvent func(input.Event) Msg) Node {
	return Component(nil, listProps{
		items: items, selected: selected,
		style: style, selectedStyle: selectedStyle,
		onEvent: onEvent,
	}, func() Widget {
		return &listWidget{}
	})
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
	_, height := p.Size()
	if height <= 0 || len(w.items) == 0 {
		return
	}

	switch {
	case w.selected < w.scrollOffset:
		w.scrollOffset = w.selected
	case w.selected >= w.scrollOffset+height:
		w.scrollOffset = w.selected - height + 1
	}
	if maxOffset := max(len(w.items)-height, 0); w.scrollOffset > maxOffset {
		w.scrollOffset = maxOffset
	}
	if w.scrollOffset < 0 {
		w.scrollOffset = 0
	}

	for row := range height {
		idx := w.scrollOffset + row
		if idx >= len(w.items) {
			break
		}
		style := w.style
		prefix := "  "
		if idx == w.selected {
			style = w.selectedStyle
			prefix = ". "
			if w.focused {
				prefix = "> "
			}
		}
		p.Text(0, row, prefix+w.items[idx], style)
	}
}

func (w *listWidget) HandleEvent(e input.Event) Cmd {
	if w.onEvent == nil {
		return nil
	}
	msg := w.onEvent(e)
	if msg == nil {
		return nil
	}
	return func() Msg { return msg }
}

func (w *listWidget) Focusable() bool         { return true }
func (w *listWidget) SetFocused(focused bool) { w.focused = focused }
