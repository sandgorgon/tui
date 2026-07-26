package tui

import "github.com/sandgorgon/tui/cell"

// FocusScope is implemented by a Widget that hosts its own nested
// focus order and, while active, should claim Tab/Shift-Tab
// traversal exclusively — e.g. Modal, which needs Tab to cycle among
// the widgets in its body rather than treating the whole modal as one
// focusable unit, and needs the background made unreachable while
// it's open (docs/DESIGN.md's "modal focus scoping", deferred from M8
// to here). A widget implementing FocusScope should return false from
// its own Focusable(): the scope's *contents*, not the wrapping
// widget itself, are what becomes focusable.
//
// At most one active scope is used at a time; if more than one
// mounted widget reports Active() == true, the App uses whichever it
// finds first in document order (nesting one FocusScope inside
// another isn't a supported v1 use case).
type FocusScope interface {
	// Active reports whether this scope currently claims exclusive
	// focus traversal — e.g. a Modal reports true only while open.
	Active() bool
	// Focusables returns the focusable widgets within this scope's own
	// content, in the order Tab should cycle through them.
	Focusables() []Widget
}

// OverlayPainter is implemented by a widget that needs to paint on top
// of the rest of the frame after everything else has been drawn,
// rather than into whatever (non-overlapping) Rect a parent Box would
// otherwise assign it — e.g. Modal, which must cover its siblings, not
// sit alongside them the way Box's layout.Split-based children do.
// Only meaningful on a widget that's also the currently active
// FocusScope: once per frame, after the main tree is painted, the App
// gives the full frame buffer to whichever active scope's widget
// implements this. Such a widget's regular Paint is typically a no-op
// — see Modal's doc comment (package widget) for why.
type OverlayPainter interface {
	PaintOverlay(p *cell.Painter)
}

// findActiveFocusScope walks r in document order for a mounted widget
// implementing FocusScope with Active() == true, or nil if none.
func findActiveFocusScope(r *retained) FocusScope {
	if r == nil {
		return nil
	}
	if r.kind == kindWidget {
		if scope, ok := r.widget.(FocusScope); ok && scope.Active() {
			return scope
		}
		return nil
	}
	for _, c := range r.children {
		if scope := findActiveFocusScope(c); scope != nil {
			return scope
		}
	}
	return nil
}
