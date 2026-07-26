package widget

import (
	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

// SelectOptions configures Select.
type SelectOptions struct {
	Theme style.Theme

	// Open is whether the dropdown list is currently expanded.
	Open bool
}

// Select is a focusable dropdown/combo box: closed, it shows the
// currently selected option on one line; open (opts.Open), it expands
// downward into a scrollable option list, reusing List's exact
// cursor/scroll behavior. Unlike a typical GUI dropdown, it doesn't
// float an overlay on top of other content: package widget's only
// overlay mechanism (tui.FocusScope/OverlayPainter) is built for
// Modal/CommandPalette's full-screen-scrim case, and doesn't (yet)
// give a widget any way to learn its own absolute screen position,
// which an anchored popup would need. So instead, the caller reserves
// enough vertical space up front for Select's Node — however many
// rows it wants visible when open, plus 1 for the closed control — and
// Select uses that same space differently depending on Open, rather
// than growing past its assigned Rect.
//
// options, selected, and cursor are all caller-owned business state
// (docs/DESIGN.md §3.1), passed in fresh every frame — exactly like
// List, which Select's open-state list behaves the same as: cursor is
// what Up/Down/onEvent moves, selected is what's actually chosen
// (typically changed on Enter, via onEvent), and Select retains only
// scrollOffset for the open list, the same ephemeral state List keeps.
func Select(options []string, selected int, cursor int, opts SelectOptions, onEvent func(input.Event) tui.Msg) tui.Node {
	return tui.Component(nil, selectProps{
		options: options, selected: selected, cursor: cursor, opts: opts, onEvent: onEvent,
	}, func() tui.Widget {
		return &selectWidget{}
	})
}

type selectProps struct {
	options  []string
	selected int
	cursor   int
	opts     SelectOptions
	onEvent  func(input.Event) tui.Msg
}

type selectWidget struct {
	selectProps
	focused      bool
	scrollOffset int
}

func (w *selectWidget) Reconcile(props any) bool {
	w.selectProps = props.(selectProps)
	return true
}

func (w *selectWidget) Paint(p *cell.Painter) {
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
	innerW, innerH := inner.Size()
	if innerW <= 0 || innerH <= 0 {
		return
	}

	base := w.opts.Theme.Text()
	label := "(none)"
	if w.selected >= 0 && w.selected < len(w.options) {
		label = w.options[w.selected]
	}
	arrow := "▾ "
	if w.opts.Open {
		arrow = "▴ "
	}
	inner.Text(0, 0, arrow+label, base)

	if !w.opts.Open || innerH <= 1 || len(w.options) == 0 {
		return
	}
	listHeight := innerH - 1
	w.scrollOffset = clampScroll(w.scrollOffset, w.cursor, len(w.options), listHeight)

	selected := cell.Style{Fg: base.Fg, Bg: base.Bg, Attr: base.Attr | cell.AttrReverse}
	for row := range listHeight {
		idx := w.scrollOffset + row
		if idx >= len(w.options) {
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
		check := "  "
		if idx == w.selected {
			check = "• "
		}
		inner.Text(0, row+1, marker+check+w.options[idx], rowStyle)
	}
}

func (w *selectWidget) HandleEvent(e input.Event) tui.Cmd {
	if w.onEvent == nil {
		return nil
	}
	msg := w.onEvent(e)
	if msg == nil {
		return nil
	}
	return func() tui.Msg { return msg }
}

func (w *selectWidget) Focusable() bool         { return true }
func (w *selectWidget) SetFocused(focused bool) { w.focused = focused }
