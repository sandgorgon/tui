package tui

import (
	"reflect"

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
	widget    Widget
	propsType reflect.Type // the concrete type of the props last handed to widget.Reconcile
}

// reconcile updates prev in place to match next, constructing a fresh
// retained node (and, for a widget, a fresh Widget instance) whenever
// this tree slot didn't match anything reusable from the previous
// frame: prev is nil, next.kind differs, or (kindWidget only) next's
// props are a different concrete type than what the retained Widget
// was last reconciled with.
//
// That last check matters because kindWidget is one flat tag shared by
// every widget.Xxx constructor in package widget — Paragraph, List,
// Table, all of it. Without it, an unkeyed tree slot that renders a
// different widget across frames (e.g. Tabs-driven page content: page
// 0 is a Paragraph, page 1 is a List, both at the same position) would
// have reconcile reuse the old frame's *paragraphWidget and hand it
// the new frame's listProps, which panics inside that widget's own
// Reconcile (a plain props.(paragraphProps) type assertion) rather
// than failing safely. Each widget's props type is unique to that
// widget (paragraphProps, listProps, ...), so comparing
// reflect.TypeOf(props) is a cheap, reliable stand-in for "is this
// still the same kind of widget" with no change needed to Widget or
// Node itself. This is a real, easy-to-hit case (any conditionally-
// rendered branch without an explicit Node.Key), not a hypothetical:
// found via examples/gallery's Tabs-switched pages, M12.
func reconcile(prev *retained, next Node) *retained {
	mismatch := prev == nil || prev.kind != next.kind
	if !mismatch && next.kind == kindWidget && prev.propsType != reflect.TypeOf(next.props) {
		mismatch = true
	}
	if mismatch {
		if prev != nil {
			disposeTree(prev)
		}
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
		prev.propsType = reflect.TypeOf(next.props)
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
// Any prev child left unmatched — no longer present in next at all —
// is disposed (see dispose.go).
func reconcileChildren(prev []*retained, next []BoxChild) []*retained {
	byKey := make(map[any]*retained, len(prev))
	for _, p := range prev {
		if p.key != nil {
			byKey[p.key] = p
		}
	}

	out := make([]*retained, len(next))
	used := make(map[*retained]bool, len(next))
	for i, nc := range next {
		var p *retained
		if nc.Node.key != nil {
			p = byKey[nc.Node.key]
		} else if i < len(prev) && prev[i].key == nil {
			p = prev[i]
		}
		if p != nil {
			used[p] = true
		}
		out[i] = reconcile(p, nc.Node)
	}

	for _, p := range prev {
		if !used[p] {
			disposeTree(p)
		}
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

// collectFocusables returns the Focusable widgets Tab/Shift-Tab should
// cycle through, in document order. If any mounted widget within r
// implements FocusScope and reports Active() == true, traversal is
// scoped exclusively to that widget's own Focusables (see FocusScope's
// doc comment) — the first such active scope found in document order
// wins, and everything outside it is ignored.
func collectFocusables(r *retained) []Widget {
	if scope := findActiveFocusScope(r); scope != nil {
		return scope.Focusables()
	}
	return collectPlainFocusables(r)
}

// collectPlainFocusables is collectFocusables' unscoped case: every
// kindWidget node whose Widget reports itself Focusable, found by
// walking r in document order.
func collectPlainFocusables(r *retained) []Widget {
	if r == nil {
		return nil
	}
	if r.kind == kindWidget {
		if r.widget.Focusable() {
			return []Widget{r.widget}
		}
		return nil
	}
	var out []Widget
	for _, c := range r.children {
		out = append(out, collectPlainFocusables(c)...)
	}
	return out
}
