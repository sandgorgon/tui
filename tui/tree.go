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
