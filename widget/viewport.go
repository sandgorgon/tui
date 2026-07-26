package widget

import (
	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/tui"
)

// Viewport scrolls a child Node too tall to fit the space it's
// painted into. No widget in this library measures its own intrinsic
// size — layout stays a one-pass solver driven by explicit constraints
// (docs/DESIGN.md §3.3), not per-widget measurement — so the caller
// supplies contentHeight explicitly: however many rows child naturally
// needs (e.g. the number of lines a Paragraph would wrap to).
//
// Scroll position is Up/Down/PgUp/PgDn/Home/End-driven and handled
// entirely inside Viewport's own HandleEvent, with no onEvent callback
// to the application at all — unlike List's cursor, "how far scrolled"
// has no business meaning, so it's a clean case of state that's purely
// ephemeral (docs/DESIGN.md §3.1) and never needs to leave the widget.
func Viewport(child tui.Node, contentHeight int) tui.Node {
	return tui.Component(nil, viewportProps{child: child, contentHeight: contentHeight}, func() tui.Widget {
		return &viewportWidget{}
	})
}

type viewportProps struct {
	child         tui.Node
	contentHeight int
}

type viewportWidget struct {
	viewportProps
	content tui.Tree
	buf     *cell.Buffer

	scrollOffset int
	// lastHeight/lastContentHeight are the most recent Paint's visible
	// and virtual content heights, remembered so HandleEvent (PgUp/
	// PgDn/End) has something to compute against — HandleEvent can run
	// before the next Paint (e.g. several keys arriving between
	// frames), so this can't just be read out of the current painter.
	lastHeight, lastContentHeight int
}

func (w *viewportWidget) Reconcile(props any) bool {
	w.viewportProps = props.(viewportProps)
	return true
}

func (w *viewportWidget) Paint(p *cell.Painter) {
	width, height := p.Size()
	if width <= 0 || height <= 0 {
		return
	}

	contentH := max(w.contentHeight, height)
	if w.buf == nil || w.buf.Width != width || w.buf.Height != contentH {
		w.buf = cell.NewBuffer(width, contentH)
	} else {
		w.buf.Clear(cell.Style{})
	}
	w.content.Reconcile(w.child)
	w.content.Paint(cell.NewPainter(w.buf))

	if max := contentH - height; w.scrollOffset > max {
		w.scrollOffset = max
	}
	if w.scrollOffset < 0 {
		w.scrollOffset = 0
	}
	w.lastHeight, w.lastContentHeight = height, contentH

	for y := range height {
		srcY := w.scrollOffset + y
		if srcY >= contentH {
			break
		}
		for x := range width {
			p.SetRawCell(x, y, w.buf.At(x, srcY))
		}
	}
}

func (w *viewportWidget) HandleEvent(e input.Event) tui.Cmd {
	page := max(w.lastHeight, 1)
	switch ev := e.(type) {
	case input.KeyEvent:
		switch ev.Key {
		case input.KeyUp:
			w.scrollOffset--
		case input.KeyDown:
			w.scrollOffset++
		case input.KeyPgUp:
			w.scrollOffset -= page
		case input.KeyPgDown:
			w.scrollOffset += page
		case input.KeyHome:
			w.scrollOffset = 0
		case input.KeyEnd:
			w.scrollOffset = w.lastContentHeight
		}
	case input.MouseEvent:
		// Wheel scroll needs no click/hit-testing infrastructure — it
		// targets whichever widget is already focused, the same as a
		// key press. Click-based interaction (e.g. click a row to jump
		// to it) is a separate, not-yet-built capability: it needs the
		// App to track each widget's painted Rect to hit-test a
		// MouseEvent's (X,Y) against, which nothing in this library
		// does yet.
		switch ev.Button {
		case input.MouseWheelUp:
			w.scrollOffset -= 3
		case input.MouseWheelDown:
			w.scrollOffset += 3
		}
	}
	return nil
}

func (w *viewportWidget) Focusable() bool         { return true }
func (w *viewportWidget) SetFocused(focused bool) {}

// Close disposes content (see tui.Tree.Close), in case child wraps
// something that itself needs cleanup.
func (w *viewportWidget) Close() error {
	return w.content.Close()
}
