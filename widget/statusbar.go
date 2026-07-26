package widget

import (
	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/tui"
)

// Segment is one piece of a StatusBar: text plus its own style. Bg is
// always overridden with the StatusBar's own background (see
// StatusBar's background parameter) so segments don't need to repeat
// it — set Fg/Attr/Underline for anything else.
type Segment struct {
	Text  string
	Style cell.Style
}

// StatusBar is a single-line bar with independently left-, center-,
// and right-aligned groups of Segments — mode/filename on the left, a
// message centered, cursor position on the right, the classic editor/
// tmux status-line layout. It has no interactive behavior and no
// retained state: alignment depends on the width it's assigned, not
// known until Paint (see Paragraph's doc comment for the same
// reasoning), so it's recomputed fresh every frame.
func StatusBar(left, center, right []Segment, background cell.Style) tui.Node {
	return tui.Component(nil, statusBarProps{
		left: left, center: center, right: right, background: background,
	}, func() tui.Widget {
		return &statusBarWidget{}
	})
}

type statusBarProps struct {
	left, center, right []Segment
	background          cell.Style
}

type statusBarWidget struct {
	statusBarProps
}

func (w *statusBarWidget) Reconcile(props any) bool {
	w.statusBarProps = props.(statusBarProps)
	return true
}

func (w *statusBarWidget) Paint(p *cell.Painter) {
	width, height := p.Size()
	if width <= 0 || height <= 0 {
		return
	}
	p.Fill(0, 0, width, height, ' ', w.background)

	leftEnd := 0
	for _, seg := range w.left {
		leftEnd += p.Text(leftEnd, 0, seg.Text, w.styled(seg))
	}

	rightWidth := 0
	for _, seg := range w.right {
		rightWidth += stringWidth(seg.Text)
	}
	rightStart := max(width-rightWidth, leftEnd)
	col := rightStart
	for _, seg := range w.right {
		col += p.Text(col, 0, seg.Text, w.styled(seg))
	}

	centerWidth := 0
	for _, seg := range w.center {
		centerWidth += stringWidth(seg.Text)
	}
	centerStart := (width - centerWidth) / 2
	centerStart = max(centerStart, leftEnd)
	centerStart = min(centerStart, rightStart-centerWidth)
	centerStart = max(centerStart, leftEnd)

	// Center is painted last and would otherwise be free to overwrite
	// right's already-painted cells whenever centerWidth doesn't fit in
	// the [leftEnd, rightStart) gap (e.g. a long message with a narrow
	// bar) — clip it to that gap so it's silently truncated instead.
	gap := max(rightStart-leftEnd, 0)
	centerPainter := p.Clip(cell.Rect{X: leftEnd, Y: 0, W: gap, H: height})
	col = centerStart - leftEnd
	for _, seg := range w.center {
		col += centerPainter.Text(col, 0, seg.Text, w.styled(seg))
	}
}

// styled returns seg's style with Bg forced to the bar's background,
// per Segment's doc comment.
func (w *statusBarWidget) styled(seg Segment) cell.Style {
	s := seg.Style
	s.Bg = w.background.Bg
	return s
}

func (w *statusBarWidget) HandleEvent(input.Event) tui.Cmd { return nil }
func (w *statusBarWidget) Focusable() bool                 { return false }
func (w *statusBarWidget) SetFocused(bool)                 {}
