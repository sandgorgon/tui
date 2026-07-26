package widget

import (
	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

// RadioGroup is a focusable, single-select vertical list of mutually
// exclusive options, rendered "(•) label" / "( ) label". Unlike List,
// it has no scrolling machinery — an option group is expected to be a
// short, fully-visible list — and so no ephemeral state of its own at
// all: selected is caller-owned business state, passed in fresh every
// frame, exactly like List's cursor (docs/DESIGN.md §3.1). Arrow keys
// directly change which option is selected, the standard radio-button
// convention (no separate "confirm" step): onEvent receives every
// input event while RadioGroup is focused and decides what Msg, if
// any, that becomes — the same onEvent contract as every other widget
// in this package.
func RadioGroup(options []string, selected int, theme style.Theme, onEvent func(input.Event) tui.Msg) tui.Node {
	return tui.Component(nil, radioGroupProps{
		options: options, selected: selected, theme: theme, onEvent: onEvent,
	}, func() tui.Widget {
		return &radioGroupWidget{}
	})
}

type radioGroupProps struct {
	options  []string
	selected int
	theme    style.Theme
	onEvent  func(input.Event) tui.Msg
}

type radioGroupWidget struct {
	radioGroupProps
	focused bool
}

func (w *radioGroupWidget) Reconcile(props any) bool {
	w.radioGroupProps = props.(radioGroupProps)
	return true
}

func (w *radioGroupWidget) Paint(p *cell.Painter) {
	_, height := p.Size()
	base := w.theme.Text()
	for row, opt := range w.options {
		if row >= height {
			break
		}
		marker := "( ) "
		style := base
		if row == w.selected {
			marker = "(•) "
			style = cell.Style{Fg: w.theme.Primary, Bg: base.Bg, Attr: cell.AttrBold}
		}
		if row == w.selected && w.focused {
			style.Underline = cell.UnderlineSingle
		}
		p.Text(0, row, marker+opt, style)
	}
}

func (w *radioGroupWidget) HandleEvent(e input.Event) tui.Cmd {
	// RadioGroup draws no border and doesn't scroll, unlike List — a
	// MouseEvent's Y (local to the widget's full bounds, per App's
	// hit-testing) is already the option index with no translation
	// needed; only bounds-check it.
	if me, ok := e.(input.MouseEvent); ok && (me.Y < 0 || me.Y >= len(w.options)) {
		return nil
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

func (w *radioGroupWidget) Focusable() bool         { return true }
func (w *radioGroupWidget) SetFocused(focused bool) { w.focused = focused }
