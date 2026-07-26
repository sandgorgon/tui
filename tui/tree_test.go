package tui

import (
	"testing"

	"github.com/sandgorgon/tui/cell"
)

func TestTreeReconcileAndPaint(t *testing.T) {
	var tr Tree
	tr.Reconcile(Text("a", cell.Style{}))

	buf := cell.NewBuffer(3, 1)
	tr.Paint(cell.NewPainter(buf))
	if got := buf.String(); got != "a  " {
		t.Errorf("Buffer = %q, want %q", got, "a  ")
	}

	tr.Reconcile(Text("bb", cell.Style{}))
	buf2 := cell.NewBuffer(3, 1)
	tr.Paint(cell.NewPainter(buf2))
	if got := buf2.String(); got != "bb " {
		t.Errorf("Buffer after update = %q, want %q", got, "bb ")
	}
}

func TestTreeRetainsWidgetInstanceAcrossReconcile(t *testing.T) {
	var mounts int
	w := &fakeWidget{}
	factory := func() Widget { mounts++; return w }

	var tr Tree
	tr.Reconcile(Component("k", 1, factory))
	tr.Reconcile(Component("k", 2, factory))

	if mounts != 1 {
		t.Errorf("mounts = %d, want 1 (factory must not rerun on update)", mounts)
	}
	if w.reconciles != 2 {
		t.Errorf("widget.Reconcile called %d times, want 2", w.reconciles)
	}
}
