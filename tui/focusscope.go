package tui

import (
	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
)

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

// OverlayBounds is implemented by a FocusScope+OverlayPainter widget
// that can report the absolute on-screen Rect of its own last-painted
// overlay — e.g. Modal, CommandPalette. Without it, App.HandleInput has
// no way to tell a click landing outside the overlay from one landing
// inside it: hitTest never matches anything while a scope is active
// (collectRects only walks the background tree, see its doc comment),
// so every click, wherever it lands on screen, would otherwise reach
// whichever of the scope's own Focusables currently holds focus with
// raw, absolute coordinates — a real bug for a widget whose body reacts
// to MouseEvent at all. App.HandleInput checks for this interface and
// withholds delivery to the focused widget when the click falls outside
// the reported bounds; see OutsideClicker for reacting to that click
// instead of merely absorbing it.
type OverlayBounds interface {
	// OverlayBounds returns the absolute Rect last painted by
	// PaintOverlay, or ok=false if nothing has been painted yet (in
	// practice this only happens before this scope's first render,
	// since PaintOverlay runs every frame this scope reports Active).
	OverlayBounds() (r cell.Rect, ok bool)
}

// OutsideClicker is implemented by an active FocusScope that also wants
// to react to a mouse click landing outside its own OverlayBounds
// (e.g. closing the overlay) rather than just having App.HandleInput
// silently withhold it. me is in absolute screen coordinates, the same
// space OverlayBounds reports in.
type OutsideClicker interface {
	HandleOutsideClick(me input.MouseEvent) Cmd
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
