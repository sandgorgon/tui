package widget

import (
	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

// Tabs is a single-line, focusable strip of tab labels with one
// active. It renders only the strip — showing whatever content belongs
// to the active tab is the caller's job (typically a Box placed below
// it), since, like List's cursor, which tab is active is business
// state Tabs takes as a prop rather than retaining itself.
func Tabs(labels []string, active int, theme style.Theme, onEvent func(input.Event) tui.Msg) tui.Node {
	return tui.Component(nil, tabsProps{
		labels: labels, active: active, theme: theme, onEvent: onEvent,
	}, func() tui.Widget {
		return &tabsWidget{}
	})
}

type tabsProps struct {
	labels  []string
	active  int
	theme   style.Theme
	onEvent func(input.Event) tui.Msg
}

type tabsWidget struct {
	tabsProps
	focused bool
}

func (w *tabsWidget) Reconcile(props any) bool {
	w.tabsProps = props.(tabsProps)
	return true
}

func (w *tabsWidget) Paint(p *cell.Painter) {
	width, height := p.Size()
	if width <= 0 || height <= 0 {
		return
	}

	col := 0
	for i, label := range w.labels {
		text := " " + label + " "

		var tabStyle cell.Style
		switch {
		case i == w.active && w.focused:
			tabStyle = cell.Style{Fg: w.theme.Background, Bg: w.theme.Primary, Attr: cell.AttrBold}
		case i == w.active:
			tabStyle = cell.Style{Fg: w.theme.Primary, Attr: cell.AttrBold, Underline: cell.UnderlineSingle}
		default:
			tabStyle = w.theme.MutedText()
		}

		col += p.Text(col, 0, text, tabStyle)
	}
}

func (w *tabsWidget) HandleEvent(e input.Event) tui.Cmd {
	if w.onEvent == nil {
		return nil
	}
	msg := w.onEvent(e)
	if msg == nil {
		return nil
	}
	return func() tui.Msg { return msg }
}

func (w *tabsWidget) Focusable() bool         { return true }
func (w *tabsWidget) SetFocused(focused bool) { w.focused = focused }
