package tui

import (
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/layout"
)

// benchTree builds a moderately nested Box tree (5 rows, one of them
// a 3-column row) of plain Text/Focusable leaves — no package widget
// dependency (tui is lower in the layering than widget), but the same
// shape of tree a real app's View() produces.
func benchTree() Node {
	row := func(label string) BoxChild {
		return Child(layout.Length(1), Text(label, cell.Style{}))
	}
	return Box(layout.Vertical,
		row("header"),
		Child(layout.Fill(1), Box(layout.Horizontal,
			Child(layout.Fill(1), Focusable("a", Text("column a", cell.Style{}), nil)),
			Child(layout.Fill(1), Focusable("b", Text("column b", cell.Style{}), nil)),
			Child(layout.Fill(1), Focusable("c", Text("column c", cell.Style{}), nil)),
		)),
		row("status 1"),
		row("status 2"),
		row("footer"),
	).Gap(1).Margin(1)
}

// BenchmarkTreeReconcileCold measures mounting benchTree from scratch
// each iteration — the cost App.NewApp/first-frame Reconcile pays.
func BenchmarkTreeReconcileCold(b *testing.B) {
	for b.Loop() {
		var tr Tree
		tr.Reconcile(benchTree())
	}
}

// BenchmarkTreeReconcileWarm measures re-reconciling the same shaped
// tree against an already-mounted one — the steady-state cost paid on
// every redraw, where the reconciler's key-or-position matching is
// expected to reuse retained widgets rather than remount them.
func BenchmarkTreeReconcileWarm(b *testing.B) {
	var tr Tree
	tr.Reconcile(benchTree())
	for b.Loop() {
		tr.Reconcile(benchTree())
	}
}

// BenchmarkTreePaint measures painting an already-reconciled tree —
// isolating Paint's cost from Reconcile's.
func BenchmarkTreePaint(b *testing.B) {
	var tr Tree
	tr.Reconcile(benchTree())
	buf := cell.NewBuffer(60, 20)
	p := cell.NewPainter(buf)
	for b.Loop() {
		tr.Paint(p)
	}
}
