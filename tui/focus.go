package tui

import (
	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
)

// focusBorderStyle and focusBorderStyleActive are Focusable's minimal
// built-in focus indicator. The real widget catalog (M10) will likely
// grow a proper Border/focus-ring widget; this is a small placeholder
// so Tab/Shift-Tab traversal is visibly demonstrable before then.
var (
	focusBorderStyle       = cell.Style{Fg: cell.ANSIColor(8)} // dim
	focusBorderStyleActive = cell.Style{Fg: cell.ANSIColor(6), Attr: cell.AttrBold}
)

type focusableProps struct {
	child   Node
	onEvent func(input.Event) Msg
}

// Focusable wraps child so it participates in Tab/Shift-Tab focus
// traversal (see focus tracking in app.go): while it holds focus,
// input events the App would otherwise deliver to it are passed to
// onEvent, and any non-nil Msg it returns is fed back into the App's
// event loop exactly like a Cmd's result. key follows Node.Key's
// matching rule.
//
// A one-cell border is always reserved around child so the border's
// presence doesn't shift child's layout when focus changes — only its
// style does.
func Focusable(key any, child Node, onEvent func(input.Event) Msg) Node {
	return Component(key, focusableProps{child: child, onEvent: onEvent}, func() Widget {
		return &focusableWidget{}
	})
}

type focusableWidget struct {
	child   *retained
	onEvent func(input.Event) Msg
	focused bool
}

func (w *focusableWidget) Reconcile(props any) bool {
	p := props.(focusableProps)
	w.onEvent = p.onEvent
	w.child = reconcile(w.child, p.child)
	return true
}

func (w *focusableWidget) Paint(p *cell.Painter) {
	width, height := p.Size()
	style := focusBorderStyle
	if w.focused {
		style = focusBorderStyleActive
	}
	drawBorder(p, width, height, style)

	inner := p.Clip(cell.Rect{X: 1, Y: 1, W: max(width-2, 0), H: max(height-2, 0)})
	w.child.paint(inner)
}

func (w *focusableWidget) HandleEvent(e input.Event) Cmd {
	if w.onEvent == nil {
		return nil
	}
	msg := w.onEvent(e)
	if msg == nil {
		return nil
	}
	return func() Msg { return msg }
}

func (w *focusableWidget) Focusable() bool         { return true }
func (w *focusableWidget) SetFocused(focused bool) { w.focused = focused }

// drawBorder paints a single-line box border in style around the
// [0,0)-[width,height) rectangle of p.
func drawBorder(p *cell.Painter, width, height int, style cell.Style) {
	if width < 2 || height < 2 {
		return
	}
	p.SetCell(0, 0, '┌', style)
	p.SetCell(width-1, 0, '┐', style)
	p.SetCell(0, height-1, '└', style)
	p.SetCell(width-1, height-1, '┘', style)
	for x := 1; x < width-1; x++ {
		p.SetCell(x, 0, '─', style)
		p.SetCell(x, height-1, '─', style)
	}
	for y := 1; y < height-1; y++ {
		p.SetCell(0, y, '│', style)
		p.SetCell(width-1, y, '│', style)
	}
}
