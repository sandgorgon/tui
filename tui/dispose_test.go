package tui

import (
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/layout"
)

type closerWidget struct {
	fakeWidget
	closed bool
}

func (w *closerWidget) Close() error {
	w.closed = true
	return nil
}

func TestReconcileChildrenDisposesDroppedChild(t *testing.T) {
	cw := &closerWidget{}
	tree := func(showExtra bool) Node {
		children := []BoxChild{
			Child(layout.Fill(1), Component("keep", nil, func() Widget { return &fakeWidget{} })),
		}
		if showExtra {
			children = append(children, Child(layout.Fill(1), Component("drop-me", nil, func() Widget { return cw })))
		}
		return Box(layout.Horizontal, children...)
	}

	r := reconcile(nil, tree(true))
	if len(r.children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(r.children))
	}

	reconcile(r, tree(false))
	if !cw.closed {
		t.Error("widget dropped from the tree should have been Closed")
	}
}

func TestReconcileKindChangeDisposesReplacedWidget(t *testing.T) {
	cw := &closerWidget{}
	r := reconcile(nil, Component("k", nil, func() Widget { return cw }))

	reconcile(r, Text("now text instead", cell.Style{}))
	if !cw.closed {
		t.Error("widget replaced by a different-kind Node should have been Closed")
	}
}

func TestReconcileSameKindDoesNotDispose(t *testing.T) {
	cw := &closerWidget{}
	r := reconcile(nil, Component("k", 1, func() Widget { return cw }))
	reconcile(r, Component("k", 2, func() Widget { return cw }))

	if cw.closed {
		t.Error("a widget that's still present (matched by key) should not have been Closed")
	}
}

// TestReconcileReparentingDoesNotDisposeReusedWidget is the issue #3 /
// reconciler-cross-parent-key-reuse.md scenario end to end: splitting
// a pane wraps its live widget one level deeper under a brand-new
// sibling Box. The widget must survive (not get Closed) since it's
// still present, just reparented -- for a real widget.Terminal this is
// what keeps its pty alive across a split instead of killing it.
func TestReconcileReparentingDoesNotDisposeReusedWidget(t *testing.T) {
	cw := &closerWidget{}
	frame1 := Box(layout.Vertical,
		Child(layout.Fill(1), Component("leaf", nil, func() Widget { return cw })),
	)
	r := reconcile(nil, frame1)

	frame2 := Box(layout.Vertical,
		Child(layout.Fill(1), Box(layout.Horizontal,
			Child(layout.Fill(1), Component("leaf", nil, func() Widget { return cw })),
			Child(layout.Fill(1), Component("new", nil, func() Widget { return &fakeWidget{} })),
		).Key("wrapper")),
	)
	r = reconcile(r, frame2)

	if cw.closed {
		t.Fatal("reparented widget should not have been Closed -- it's still live, just moved to a new parent")
	}
	if r.children[0].children[0].widget != Widget(cw) {
		t.Fatal("expected the reparented widget instance in its new position")
	}
}

func TestFocusableCloseDisposesChild(t *testing.T) {
	cw := &closerWidget{}
	child := Component("inner", nil, func() Widget { return cw })
	r := reconcile(nil, Focusable("f", child, nil))

	if err := r.widget.(*focusableWidget).Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !cw.closed {
		t.Error("Focusable.Close should dispose its wrapped child")
	}
}

func TestTreeCloseDisposesContent(t *testing.T) {
	cw := &closerWidget{}
	var tr Tree
	tr.Reconcile(Component("inner", nil, func() Widget { return cw }))

	if err := tr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !cw.closed {
		t.Error("Tree.Close should dispose its content")
	}
}

func TestAppCloseDisposesTree(t *testing.T) {
	cw := &closerWidget{}
	m := &widgetHostModel{node: Component("inner", nil, func() Widget { return cw })}
	app := NewApp(m, 10, 5)

	if err := app.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !cw.closed {
		t.Error("App.Close should dispose the current Node tree")
	}
}

// widgetHostModel is a minimal Model that always shows a single fixed
// Node, reused by the App-level dispose/close tests here (a version
// of the same helper package widget's tests define for itself, since
// tui's own tests can't import package widget without a cycle).
type widgetHostModel struct{ node Node }

func (m *widgetHostModel) Init() Cmd               { return nil }
func (m *widgetHostModel) Update(Msg) (Model, Cmd) { return m, nil }
func (m *widgetHostModel) View() Node              { return m.node }
