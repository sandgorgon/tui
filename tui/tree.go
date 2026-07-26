package tui

import "github.com/sandgorgon/tui/cell"

// Tree is an opaque, retained sub-tree a Widget can use to host and
// reconcile an arbitrary child Node across frames — the same
// mechanism Box uses internally for its own children (reconcile.go),
// exposed so a Widget defined outside package tui (see package widget,
// M10 on) can wrap child content too, e.g. a scrollable Viewport or a
// future Modal. The zero Tree is ready to use.
type Tree struct {
	root *retained
}

// Reconcile updates t in place to match next — mounting on the first
// call, exactly like the reconciler does for a Box's children.
func (t *Tree) Reconcile(next Node) {
	t.root = reconcile(t.root, next)
}

// Paint draws t's current content into p.
func (t *Tree) Paint(p *cell.Painter) {
	t.root.paint(p)
}

// Focusables returns the Focusable widgets within t's current content,
// in document order — for a Widget that implements FocusScope by
// hosting its content in a Tree (e.g. widget.Modal) to implement its
// own Focusables method by delegating to this.
func (t *Tree) Focusables() []Widget {
	return collectFocusables(t.root)
}

// Close disposes t's current content (see dispose.go) — a Widget that
// embeds a Tree to host child content (e.g. widget.Viewport) and holds
// or wraps anything needing cleanup of its own should implement
// io.Closer by delegating to this.
func (t *Tree) Close() error {
	disposeTree(t.root)
	return nil
}
