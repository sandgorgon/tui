package tui

import (
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/layout"
)

// fakeWidget is a Widget test double that records what happens to it,
// so reconcile's promise — construct once, then Reconcile in place —
// is checked directly rather than inferred from painted output.
type fakeWidget struct {
	reconciles int
	lastProps  any
	focused    bool
	focusable  bool
	events     []input.Event
	handleMsg  Msg
}

func (w *fakeWidget) Reconcile(props any) bool {
	w.reconciles++
	w.lastProps = props
	return true
}
func (w *fakeWidget) Paint(p *cell.Painter) {}
func (w *fakeWidget) HandleEvent(e input.Event) Cmd {
	w.events = append(w.events, e)
	if w.handleMsg == nil {
		return nil
	}
	msg := w.handleMsg
	return func() Msg { return msg }
}
func (w *fakeWidget) Focusable() bool         { return w.focusable }
func (w *fakeWidget) SetFocused(focused bool) { w.focused = focused }

func TestReconcileTextUpdatesInPlace(t *testing.T) {
	r1 := reconcile(nil, Text("a", cell.Style{}))
	r2 := reconcile(r1, Text("b", cell.Style{Attr: cell.AttrBold}))

	if r1 != r2 {
		t.Fatal("expected the same retained node to be reused across frames")
	}
	if r2.text != "b" {
		t.Errorf("text = %q, want %q", r2.text, "b")
	}
	if r2.style.Attr != cell.AttrBold {
		t.Errorf("style not updated: %+v", r2.style)
	}
}

func TestReconcileKindChangeMountsFresh(t *testing.T) {
	r1 := reconcile(nil, Text("a", cell.Style{}))
	r2 := reconcile(r1, Box(layout.Horizontal))

	if r1 == r2 {
		t.Fatal("expected a fresh retained node when the node kind changes")
	}
	if r2.kind != kindBox {
		t.Errorf("kind = %v, want kindBox", r2.kind)
	}
}

func TestReconcileWidgetFactoryOnlyRunsOnMount(t *testing.T) {
	var mounts int
	w := &fakeWidget{}
	factory := func() Widget { mounts++; return w }

	r := reconcile(nil, Component("k", 1, factory))
	if mounts != 1 {
		t.Fatalf("mounts after initial reconcile = %d, want 1", mounts)
	}
	if r.widget != Widget(w) {
		t.Fatal("retained widget should be the instance the factory constructed")
	}
	if w.reconciles != 1 || w.lastProps != 1 {
		t.Fatalf("widget not Reconciled with initial props: reconciles=%d lastProps=%v", w.reconciles, w.lastProps)
	}

	r2 := reconcile(r, Component("k", 2, factory))
	if r2 != r {
		t.Fatal("expected the same retained node reused across frames")
	}
	if mounts != 1 {
		t.Fatalf("mounts after second reconcile = %d, want still 1 (factory must not run again)", mounts)
	}
	if w.reconciles != 2 || w.lastProps != 2 {
		t.Fatalf("widget not Reconciled with updated props: reconciles=%d lastProps=%v", w.reconciles, w.lastProps)
	}
}

// typedPropsA/typedPropsB and strictWidget stand in for two different
// real widget.Xxx types (e.g. Paragraph and List), each of which
// type-asserts its own concrete props type inside Reconcile — the
// same thing every widget in package widget actually does (see e.g.
// paragraphWidget.Reconcile's props.(paragraphProps)). fakeWidget
// above accepts props as bare `any` and can't reproduce the bug this
// test guards against.
type typedPropsA struct{ v int }
type typedPropsB struct{ v string }

type strictWidget struct {
	wantsA     bool // true: asserts typedPropsA; false: asserts typedPropsB
	reconciles int
}

func (w *strictWidget) Reconcile(props any) bool {
	if w.wantsA {
		_ = props.(typedPropsA)
	} else {
		_ = props.(typedPropsB)
	}
	w.reconciles++
	return true
}
func (w *strictWidget) Paint(p *cell.Painter)       {}
func (w *strictWidget) HandleEvent(input.Event) Cmd { return nil }
func (w *strictWidget) Focusable() bool             { return false }
func (w *strictWidget) SetFocused(bool)             {}

// TestReconcileWidgetTypeChangeMountsFreshInsteadOfPanicking guards
// against a real crash found via examples/gallery (M12): an unkeyed
// tree slot that renders a different widget.Xxx across frames (e.g.
// Tabs-driven page content — page 0 a Paragraph, page 1 a List, both
// at the same Box-child position) used to have reconcile reuse the
// old frame's retained Widget and hand it the new frame's props,
// panicking inside that widget's own type-asserting Reconcile. See
// reconcile.go's propsType field and doc comment for the fix.
func TestReconcileWidgetTypeChangeMountsFreshInsteadOfPanicking(t *testing.T) {
	wA := &strictWidget{wantsA: true}
	wB := &strictWidget{wantsA: false}

	r := reconcile(nil, Component(nil, typedPropsA{v: 1}, func() Widget { return wA }))
	if r.widget != Widget(wA) {
		t.Fatal("initial mount didn't wire up widget A")
	}

	r2 := reconcile(r, Component(nil, typedPropsB{v: "x"}, func() Widget { return wB }))
	if r2.widget != Widget(wB) {
		t.Fatal("expected the retained node to remount to widget B instead of reusing widget A")
	}
	if wA.reconciles != 1 {
		t.Errorf("widget A should not be Reconciled again once replaced, got %d calls", wA.reconciles)
	}
	if wB.reconciles != 1 {
		t.Errorf("widget B should have been Reconciled once after mounting, got %d calls", wB.reconciles)
	}
}

func TestReconcileChildrenMatchByKeyAcrossReorder(t *testing.T) {
	wA, wB := &fakeWidget{}, &fakeWidget{}
	build := func(order []string) Node {
		widgets := map[string]*fakeWidget{"a": wA, "b": wB}
		var children []BoxChild
		for _, k := range order {
			w := widgets[k]
			children = append(children, Child(layout.Fill(1), Component(k, k, func() Widget { return w })))
		}
		return Box(layout.Vertical, children...)
	}

	r := reconcile(nil, build([]string{"a", "b"}))
	firstA, firstB := r.children[0], r.children[1]
	if firstA.widget != Widget(wA) || firstB.widget != Widget(wB) {
		t.Fatal("initial mount didn't wire up the expected widget instances")
	}

	r = reconcile(r, build([]string{"b", "a"}))
	if r.children[0] != firstB {
		t.Error("child at index 0 after reorder should be the retained node keyed \"b\", not a fresh one")
	}
	if r.children[1] != firstA {
		t.Error("child at index 1 after reorder should be the retained node keyed \"a\", not a fresh one")
	}
}

func TestReconcileChildrenPositionalWithoutKeys(t *testing.T) {
	build := func(texts []string) Node {
		var children []BoxChild
		for _, s := range texts {
			children = append(children, Child(layout.Fill(1), Text(s, cell.Style{})))
		}
		return Box(layout.Horizontal, children...)
	}

	r := reconcile(nil, build([]string{"a", "b"}))
	first := r.children[0]

	r = reconcile(r, build([]string{"c", "d"}))
	if r.children[0] != first {
		t.Error("unkeyed children should be matched by position, reusing the retained node at the same index")
	}
	if r.children[0].text != "c" {
		t.Errorf("text = %q, want %q", r.children[0].text, "c")
	}
}

func TestBoxPaintUsesLayoutSplit(t *testing.T) {
	node := Box(layout.Horizontal,
		Child(layout.Length(2), Text("ab", cell.Style{})),
		Child(layout.Fill(1), Text("xyz", cell.Style{})),
	)
	r := reconcile(nil, node)

	buf := cell.NewBuffer(6, 1)
	r.paint(cell.NewPainter(buf))

	want := "abxyz "
	if got := buf.String(); got != want {
		t.Errorf("Buffer = %q, want %q", got, want)
	}
}

func TestCollectFocusablesInDocumentOrder(t *testing.T) {
	wA := &fakeWidget{focusable: true}
	wB := &fakeWidget{focusable: false}
	wC := &fakeWidget{focusable: true}

	tree := Box(layout.Vertical,
		Child(layout.Fill(1), Component("a", nil, func() Widget { return wA })),
		Child(layout.Fill(1), Component("b", nil, func() Widget { return wB })),
		Child(layout.Fill(1), Component("c", nil, func() Widget { return wC })),
	)
	r := reconcile(nil, tree)

	got := collectFocusables(r)
	if len(got) != 2 {
		t.Fatalf("collectFocusables returned %d widgets, want 2", len(got))
	}
	if got[0] != Widget(wA) || got[1] != Widget(wC) {
		t.Error("expected only the Focusable widgets, in document order, skipping the non-focusable one")
	}
}
