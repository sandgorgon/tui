package tui

import (
	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/layout"
)

// Widget is implemented by a stateful, retained component. It's the
// extension point Component (and, starting at M10, the real widget
// catalog in package widget) plugs into — not something most
// application code constructs directly. The reconciler creates one
// Widget instance the first time its Node appears in a View() tree and
// keeps reusing that instance across frames for as long as the Node
// keeps matching the same tree slot (see reconcile.go), calling
// Reconcile with each new frame's props. That retained instance is
// where ephemeral, non-business UI state lives — scroll offset, cursor
// blink phase, an in-progress edit buffer — instead of the
// application's Model (see docs/DESIGN.md §3.1).
type Widget interface {
	// Reconcile updates the widget from props carried by a new Node
	// describing the same tree slot as the previous frame, reporting
	// whether anything visible changed. It's also called once, with
	// the Node's initial props, right after construction.
	Reconcile(props any) (changed bool)
	// Paint draws the widget into p, whose Size() is the Rect the
	// enclosing Box assigned to this Node.
	Paint(p *cell.Painter)
	// HandleEvent processes an input event delivered because this
	// widget is focused, optionally returning a Cmd. That Cmd is
	// resolved synchronously by App.HandleInput's caller (Run), not
	// run asynchronously the way a Cmd returned from Model.Update is —
	// see App.resolveWidgetCmd's doc comment. It must therefore only
	// ever repackage a Msg value already fully computed before
	// HandleEvent returns (the pattern every built-in widget's
	// OnChange/OnCursorChange/OnSubmit-style hook follows, being typed
	// func(...) Msg rather than Cmd) — never wrap real blocking work.
	HandleEvent(e input.Event) Cmd
	// Focusable reports whether this widget participates in
	// Tab/Shift-Tab focus traversal (see focus.go).
	Focusable() bool
	// SetFocused is called after every reconcile to tell the widget
	// whether it's the current Tab/Shift-Tab focus target, so Paint
	// can render a visual indicator. Called on every Focusable widget,
	// not just ones that are currently focused (so losing focus is
	// also observable).
	SetFocused(focused bool)
}

type nodeKind uint8

const (
	kindText nodeKind = iota
	kindBox
	kindWidget
)

// Node is a lightweight, immutable description of one piece of UI —
// what Model.View returns, not a widget itself. The reconciler diffs a
// new Node tree against the previous frame's and only where a node
// doesn't match anything previously seen does real work (a Widget
// construction) happen; see docs/DESIGN.md §3.1.
type Node struct {
	kind nodeKind
	key  any

	// kindText
	text  string
	style cell.Style

	// kindBox
	direction layout.Direction
	gap       int
	margin    int
	children  []BoxChild

	// kindWidget
	props     any
	newWidget func() Widget
}

// Key returns a copy of n identified by k for reconciliation instead
// of its position among siblings — needed when a list of sibling Nodes
// can be reordered, inserted into, or removed from between frames
// (e.g. List's rows), so the retained Widget instance for "the row
// about item 7" follows item 7 rather than whatever ends up at that
// row's index. Nodes without an explicit Key are matched by position.
func (n Node) Key(k any) Node {
	n.key = k
	return n
}

// Text is a single line of s in style, undecorated and unwrapped (see
// the future Paragraph widget, M10, for wrapping).
func Text(s string, style cell.Style) Node {
	return Node{kind: kindText, text: s, style: style}
}

// BoxChild pairs a child Node with the layout.Constraint that sizes it
// within its parent Box.
type BoxChild struct {
	Constraint layout.Constraint
	Node       Node
}

// Child is a convenience for building a BoxChild inline in a Box call.
func Child(c layout.Constraint, n Node) BoxChild {
	return BoxChild{Constraint: c, Node: n}
}

// Box lays out children along direction using package layout's
// constraint solver (see docs/DESIGN.md §3.3), re-run fresh every
// frame against whatever Rect the Box itself is assigned — so a Box
// tree is automatically resize-responsive with no extra code from the
// application.
func Box(direction layout.Direction, children ...BoxChild) Node {
	return Node{kind: kindBox, direction: direction, children: children}
}

// Gap returns a copy of n with g cells of empty space between adjacent
// children. Only meaningful on a Box node.
func (n Node) Gap(g int) Node {
	n.gap = g
	return n
}

// Margin returns a copy of n with m cells of empty space inset from
// every side before children are laid out. Only meaningful on a Box
// node.
func (n Node) Margin(m int) Node {
	n.margin = m
	return n
}

// Component wraps a stateful Widget in a Node: newWidget constructs
// the retained instance the first time this Node's key/position
// doesn't match anything from the previous frame, and props is handed
// to Widget.Reconcile on every frame (including the first) so the
// instance can pick up this frame's values. key follows the same
// position-vs-explicit-Key matching rule as Node.Key.
func Component(key any, props any, newWidget func() Widget) Node {
	return Node{kind: kindWidget, key: key, props: props, newWidget: newWidget}
}
