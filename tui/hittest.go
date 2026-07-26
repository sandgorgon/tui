package tui

import (
	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/layout"
)

// collectRects walks r in document order, recording each Focusable
// widget's absolute on-screen Rect into out — the same layout.Split
// geometry paint uses to position Box children, computed here purely
// to know where things ended up rather than to draw them. Used by App
// to hit-test a MouseEvent's (X,Y) against (see App.hitTest).
//
// Like collectPlainFocusables, this only sees the "plain" tree: a
// widget that hosts its own nested content privately (Viewport, or an
// active Modal/CommandPalette's FocusScope-provided widgets) is opaque
// here too, occupying whatever Rect its own Node was assigned and
// nothing more granular inside it. For an active FocusScope
// specifically, this means the Rects collected here (from the
// background tree) simply won't match any entry in App.focusables
// (which holds the scope's own widgets while active) — hitTest finds
// nothing and mouse clicks harmlessly fall through, rather than
// needing an explicit "don't hit-test while a modal is open" check.
func collectRects(r *retained, bounds cell.Rect, out map[Widget]cell.Rect) {
	if r == nil {
		return
	}
	switch r.kind {
	case kindBox:
		rects := layout.New(r.direction, r.constraints...).Gap(r.gap).Margin(r.margin).Split(layout.Rect{W: bounds.W, H: bounds.H})
		for i, child := range r.children {
			collectRects(child, cell.Rect{
				X: bounds.X + rects[i].X, Y: bounds.Y + rects[i].Y,
				W: rects[i].W, H: rects[i].H,
			}, out)
		}
	case kindWidget:
		if r.widget.Focusable() {
			out[r.widget] = bounds
		}
	}
}
