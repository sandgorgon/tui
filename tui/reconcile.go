package tui

import (
	"io"
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

// reconcile updates prev in place to match next for one top-level
// retained tree — App's own root (app.go), each widget.Tree
// (tree.go), and each Focusable's wrapped child (focus.go) all call
// this directly, each with its own independently-retained tree that
// this call's key matching never reaches beyond.
//
// Per-slot matching (constructing a fresh retained node, and for a
// widget a fresh Widget instance, whenever a slot didn't match
// anything reusable) is reconcileNode/reconcileChildren's job, scoped
// to one parent's direct children at a time — see their doc comments.
// This wrapper adds one thing on top: an explicit Node.Key can also
// match a retained node found anywhere else in this call's previous
// tree, not just among its new parent's own previous children, so a
// keyed subtree survives moving to a new parent across frames (being
// wrapped one level deeper, or pulled one level shallower) instead of
// being torn down and rebuilt from scratch. See
// docs/proposals/reconciler-cross-parent-key-reuse.md — this closes
// the gap it describes: splitting or unsplitting a pane that hosts a
// live widget.Terminal used to always kill and restart its pty,
// because the pane's Box necessarily gains or loses one level of
// nesting the moment it's split, and per-parent key matching alone
// has no way to follow a Node one level up or down.
//
// Because a keyed retained node now might be claimed by a slot
// anywhere in the tree, disposal of anything genuinely gone (see
// dispose.go's Close-on-io.Closer convention) is deferred to the very
// end of this call rather than happening inline as each Box's
// children are matched — until the whole new tree has been walked,
// there's no way to know whether a retained node left unmatched at
// the parent it used to occupy will still be claimed by some other
// slot visited later.
func reconcile(prev *retained, next Node) *retained {
	ctx := &reconcileCtx{byKey: make(map[any]*retained), claimed: make(map[*retained]bool)}
	var widgets []*retained
	snapshotPrev(prev, ctx.byKey, &widgets)

	out := reconcileNode(ctx, prev, next)

	disposeUnclaimed(widgets, ctx.claimed)
	return out
}

// reconcileCtx carries the whole-tree key index and claimed-node
// bookkeeping for a single reconcile call (see its doc comment)
// through the recursive reconcileNode/reconcileChildren walk.
type reconcileCtx struct {
	// byKey holds every keyed retained node found anywhere in prev,
	// gathered by snapshotPrev before reconcileNode mutates anything —
	// reconcileChildren consults it as a fallback once a next slot's
	// key misses its own parent's previous children. An entry is
	// deleted the moment it's claimed, so if next's tree carries a
	// genuine key collision (two different slots with the same key,
	// a caller bug either way), only the first slot visited reuses
	// the retained node; the second gets its own fresh one rather
	// than aliasing the same Widget instance into two tree positions.
	byKey map[any]*retained
	// claimed marks every retained node reused as prev for some next
	// slot during this pass — both an ordinary local match and a
	// byKey fallback match mark their node here. disposeUnclaimed
	// closes whatever prev-frame widget isn't in this set once the
	// walk finishes: never claimed anywhere means genuinely gone from
	// next, not just temporarily unmatched at its old parent.
	claimed map[*retained]bool
}

// snapshotPrev walks r — a previous frame's retained tree, not yet
// touched by this frame's reconcile — recording every keyed node into
// byKey and every kindWidget node into widgets. Both have to be
// gathered before reconcileNode starts mutating matched retained
// nodes in place: a reused retained node keeps its identity (same
// *retained pointer) but has its fields overwritten to the new
// frame's values, so anything not captured up front can no longer be
// told apart from this frame's own output by the time reconcile
// returns.
func snapshotPrev(r *retained, byKey map[any]*retained, widgets *[]*retained) {
	if r == nil {
		return
	}
	if r.key != nil {
		byKey[r.key] = r
	}
	switch r.kind {
	case kindBox:
		for _, c := range r.children {
			snapshotPrev(c, byKey, widgets)
		}
	case kindWidget:
		*widgets = append(*widgets, r)
	}
}

// disposeUnclaimed closes every widget from the previous frame that
// this reconcile pass never reused (see reconcileCtx.claimed) — the
// ones truly absent from next, not merely unmatched at the specific
// parent they used to occupy.
func disposeUnclaimed(widgets []*retained, claimed map[*retained]bool) {
	for _, r := range widgets {
		if claimed[r] {
			continue
		}
		if closer, ok := r.widget.(io.Closer); ok {
			_ = closer.Close()
		}
	}
}

// reconcileNode is reconcile's recursive worker: updates prev in
// place to match next, constructing a fresh retained node (and, for a
// widget, a fresh Widget instance) whenever this tree slot didn't
// match anything reusable — prev is nil, next.kind differs, or
// (kindWidget only) next's props are a different concrete type than
// what the retained Widget was last reconciled with.
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
func reconcileNode(ctx *reconcileCtx, prev *retained, next Node) *retained {
	mismatch := prev == nil || prev.kind != next.kind
	if !mismatch && next.kind == kindWidget && prev.propsType != reflect.TypeOf(next.props) {
		mismatch = true
	}
	if mismatch {
		prev = &retained{kind: next.kind}
		if next.kind == kindWidget {
			prev.widget = next.newWidget()
		}
	} else {
		ctx.claimed[prev] = true
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
		prev.children = reconcileChildren(ctx, prev.children, next.children)
	case kindWidget:
		prev.widget.Reconcile(next.props)
		prev.propsType = reflect.TypeOf(next.props)
	}
	return prev
}

// reconcileChildren matches each entry of next against prev: an entry
// with an explicit Node.Key is matched first to whichever prev child
// of this same parent carried the same key regardless of position (so
// reordering, inserting, or removing siblings doesn't disturb an
// unrelated sibling's retained state), falling back to ctx.byKey — a
// whole previous-tree index, see reconcile's doc comment — when no
// local match exists, so a keyed subtree that lived under a different
// parent last frame is reused here instead of rebuilt. An entry
// without a Key is matched only to the prev child at the same index,
// and only if that prev child was itself unkeyed (falling back to
// React-style position matching for keyless lists). Nothing here
// disposes a leftover prev child directly any more — reconcile's
// end-of-pass sweep disposes whatever no slot anywhere claimed.
func reconcileChildren(ctx *reconcileCtx, prev []*retained, next []BoxChild) []*retained {
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
			if p == nil {
				p = ctx.byKey[nc.Node.key]
			}
			// A key claimed by any slot this pass — whether matched
			// locally or via ctx's whole-tree fallback — must not be
			// handed out again: without this, two next slots sharing
			// a key by caller mistake would alias the same retained
			// node (and Widget instance) into two tree positions
			// instead of the second one safely mounting fresh.
			delete(ctx.byKey, nc.Node.key)
		} else if i < len(prev) && prev[i].key == nil {
			p = prev[i]
		}
		out[i] = reconcileNode(ctx, p, nc.Node)
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
	widgets, _ := collectFocusablesAndKeys(r)
	return widgets
}

// collectFocusablesAndKeys is collectFocusables' key-preserving form:
// keys[i] is the Node.Key that produced widgets[i] (nil for a
// positionally-matched, unkeyed slot), used by App.render() to report
// the focused widget's key via FocusAware (see focusscope.go). While
// an active FocusScope is in effect, keys is nil throughout —
// FocusScope.Focusables() doesn't preserve key information, since a
// scope's body (e.g. Modal/CommandPalette) lives in its own private
// tui.Tree and isn't key-addressable from here.
func collectFocusablesAndKeys(r *retained) (widgets []Widget, keys []any) {
	if scope := findActiveFocusScope(r); scope != nil {
		return scope.Focusables(), nil
	}
	return collectPlainFocusablesAndKeys(r)
}

// collectPlainFocusablesAndKeys is collectFocusablesAndKeys' unscoped
// case: every kindWidget node whose Widget reports itself Focusable,
// found by walking r in document order, alongside the Node.Key that
// produced it.
func collectPlainFocusablesAndKeys(r *retained) (widgets []Widget, keys []any) {
	if r == nil {
		return nil, nil
	}
	if r.kind == kindWidget {
		if r.widget.Focusable() {
			return []Widget{r.widget}, []any{r.key}
		}
		return nil, nil
	}
	for _, c := range r.children {
		w, k := collectPlainFocusablesAndKeys(c)
		widgets = append(widgets, w...)
		keys = append(keys, k...)
	}
	return widgets, keys
}
