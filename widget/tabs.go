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
	// ranges is each label's [start,end) column range from the last
	// Paint, for HandleEvent to hit-test a click's X against (see
	// tabAt) — the same "computed during Paint, used by a later
	// HandleEvent" pattern List/Table use for their own row/column
	// math.
	ranges []tabRange
}

type tabRange struct{ start, end int }

func (w *tabsWidget) Reconcile(props any) bool {
	w.tabsProps = props.(tabsProps)
	return true
}

func (w *tabsWidget) Paint(p *cell.Painter) {
	width, height := p.Size()
	if width <= 0 || height <= 0 {
		return
	}

	w.ranges = make([]tabRange, len(w.labels))
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

		start := col
		col += p.Text(col, 0, text, tabStyle)
		w.ranges[i] = tabRange{start, col}
	}
}

func (w *tabsWidget) HandleEvent(e input.Event) tui.Cmd {
	if me, ok := e.(input.MouseEvent); ok {
		idx, ok := w.tabAt(me.X)
		if !ok {
			return nil // clicked the empty space past the last label
		}
		translated := me
		translated.X = idx
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

// tabAt translates an X coordinate local to Tabs' full painted bounds
// into a label index, using the ranges recorded by the last Paint.
func (w *tabsWidget) tabAt(x int) (idx int, ok bool) {
	for i, r := range w.ranges {
		if x >= r.start && x < r.end {
			return i, true
		}
	}
	return 0, false
}

func (w *tabsWidget) Focusable() bool         { return true }
func (w *tabsWidget) SetFocused(focused bool) { w.focused = focused }
