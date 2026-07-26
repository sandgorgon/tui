package tui

import (
	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/layout"
)

// retained is the persistent counterpart to a Node: one retained tree
// survives across frames, mutated in place by reconcile as each new
// Node tree arrives from Model.View, so a kindWidget node's Widget
// instance (and whatever ephemeral state it owns) isn't rebuilt every
// frame — see the Widget doc comment in node.go and docs/DESIGN.md
// §3.1.
type retained struct {
	kind nodeKind
	key  any

	// kindText
	text  string
	style cell.Style

	// kindBox
	direction   layout.Direction
	gap, margin int
	constraints []layout.Constraint
	children    []*retained

	// kindWidget
	widget Widget
}

// reconcile updates prev in place to match next, constructing a fresh
// retained node (and, for a widget, a fresh Widget instance) only when
// prev is nil or describes a different kind of node than next — i.e.
// this tree slot didn't match anything from the previous frame. It
// recurses into Box's children via reconcileChildren, which is where
// position-vs-Key matching happens.
func reconcile(prev *retained, next Node) *retained {
	if prev == nil || prev.kind != next.kind {
		prev = &retained{kind: next.kind}
		if next.kind == kindWidget {
			prev.widget = next.newWidget()
		}
	}
	prev.key = next.key

	switch next.kind {
	case kindText:
		prev.text = next.text
		prev.style = next.style
	case kindBox:
		prev.direction = next.direction
		prev.gap = next.gap
		prev.margin = next.margin
		prev.constraints = make([]layout.Constraint, len(next.children))
		for i, c := range next.children {
			prev.constraints[i] = c.Constraint
		}
		prev.children = reconcileChildren(prev.children, next.children)
	case kindWidget:
		prev.widget.Reconcile(next.props)
	}
	return prev
}

// reconcileChildren matches each entry of next against prev: an entry
// with an explicit Node.Key is matched to whichever prev child carried
// the same key regardless of position (so reordering, inserting, or
// removing siblings doesn't disturb an unrelated sibling's retained
// state); an entry without a Key is matched only to the prev child at
// the same index, and only if that prev child was itself unkeyed
// (falling back to React-style position matching for keyless lists).
func reconcileChildren(prev []*retained, next []BoxChild) []*retained {
	byKey := make(map[any]*retained, len(prev))
	for _, p := range prev {
		if p.key != nil {
			byKey[p.key] = p
		}
	}

	out := make([]*retained, len(next))
	for i, nc := range next {
		var p *retained
		if nc.Node.key != nil {
			p = byKey[nc.Node.key]
		} else if i < len(prev) && prev[i].key == nil {
			p = prev[i]
		}
		out[i] = reconcile(p, nc.Node)
	}
	return out
}

// paint draws r into p, whose Size() is the Rect assigned to r by its
// parent (or, for the root, the whole frame buffer).
func (r *retained) paint(p *cell.Painter) {
	if r == nil {
		return
	}
	switch r.kind {
	case kindText:
		p.Text(0, 0, r.text, r.style)
	case kindBox:
		w, h := p.Size()
		rects := layout.New(r.direction, r.constraints...).Gap(r.gap).Margin(r.margin).Split(layout.Rect{W: w, H: h})
		for i, child := range r.children {
			child.paint(p.Clip(cell.Rect{X: rects[i].X, Y: rects[i].Y, W: rects[i].W, H: rects[i].H}))
		}
	case kindWidget:
		r.widget.Paint(p)
	}
}

// collectFocusables walks r in document order, returning every
// kindWidget node whose Widget reports itself Focusable — the order
// Tab/Shift-Tab cycle through (see focus.go).
func collectFocusables(r *retained) []*retained {
	if r == nil {
		return nil
	}
	if r.kind == kindWidget {
		if r.widget.Focusable() {
			return []*retained{r}
		}
		return nil
	}
	var out []*retained
	for _, c := range r.children {
		out = append(out, collectFocusables(c)...)
	}
	return out
}
