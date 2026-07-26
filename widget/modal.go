package widget

import (
	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

// ModalOptions configures Modal.
type ModalOptions struct {
	Theme style.Theme
	Title string

	// Open is whether the modal is currently shown.
	Open bool

	// Width and Height size the centered dialog box; <= 0, or larger
	// than the frame, means "use the whole frame" (no intrinsic sizing
	// exists to compute a natural size from body — see
	// docs/DESIGN.md §3.3).
	Width, Height int
}

// Modal is a centered dialog overlay that, while open, covers the
// rest of the frame and claims Tab/Shift-Tab focus exclusively for
// its own body content (see tui.FocusScope) — background widgets
// become unreachable until it closes. Its regular Paint is a no-op:
// all drawing happens via PaintOverlay (tui.OverlayPainter), since a
// modal must cover its siblings rather than sit alongside them the
// way a normal Box child does — see OverlayPainter's doc comment in
// package tui. Because of that, Modal's own Node can be placed
// anywhere convenient in the View() tree, even given Length(0); its
// assigned Rect is never used. It draws an opaque dialog box, not a
// translucent scrim over the background — package cell's Painter is
// write-only (see docs/DESIGN.md §3), so there's nothing to blend
// with; a solid box is the standard treatment plenty of real terminal
// dialog tools (e.g. whiptail/dialog(1)) use anyway.
//
// Open is caller-owned business state, read fresh every frame: an app
// that conditionally omits the Modal Node entirely while closed gets a
// fresh body every time it reopens (its retained state — e.g. any
// TextInput inside — is disposed the moment the Node stops appearing,
// see docs/DESIGN.md's widget disposal); always including the Node
// (with Open: false when hidden) instead keeps that state across
// opens. Either is valid; which one an app wants is its own call.
func Modal(body tui.Node, opts ModalOptions) tui.Node {
	return tui.Component(nil, modalProps{body: body, opts: opts}, func() tui.Widget {
		return &modalWidget{}
	})
}

type modalProps struct {
	body tui.Node
	opts ModalOptions
}

type modalWidget struct {
	modalProps
	content tui.Tree
}

func (w *modalWidget) Reconcile(props any) bool {
	w.modalProps = props.(modalProps)
	w.content.Reconcile(w.body)
	return true
}

// Paint is a no-op — see Modal's doc comment; all drawing happens in
// PaintOverlay, once per frame, regardless of where in the tree this
// Node's regular Paint would otherwise have been called from.
func (w *modalWidget) Paint(p *cell.Painter) {}

func (w *modalWidget) PaintOverlay(p *cell.Painter) {
	if !w.opts.Open {
		return
	}
	dialog := centeredOverlay(p, w.opts.Width, w.opts.Height)
	mw, mh := dialog.Size()
	if mw < 2 || mh < 2 {
		return
	}

	bg := w.opts.Theme.Text()
	dialog.Fill(0, 0, mw, mh, ' ', bg)
	drawBorder(dialog, mw, mh, w.opts.Theme.FocusStyle())

	if w.opts.Title != "" && mw > 4 {
		dialog.Text(2, 0, " "+w.opts.Title+" ", cell.Style{Fg: bg.Fg, Bg: bg.Bg, Attr: cell.AttrBold})
	}

	inner := dialog.Clip(cell.Rect{X: 1, Y: 1, W: max(mw-2, 0), H: max(mh-2, 0)})
	w.content.Paint(inner)
}

func (w *modalWidget) HandleEvent(input.Event) tui.Cmd { return nil }

// Focusable is false: per tui.FocusScope's doc comment, Modal's own
// *contents* become focusable while open (via Focusables below), not
// the wrapping widget itself.
func (w *modalWidget) Focusable() bool { return false }
func (w *modalWidget) SetFocused(bool) {}

func (w *modalWidget) Active() bool             { return w.opts.Open }
func (w *modalWidget) Focusables() []tui.Widget { return w.content.Focusables() }

// Close disposes body (see tui.Tree.Close), in case it wraps something
// that itself needs cleanup.
func (w *modalWidget) Close() error {
	return w.content.Close()
}
