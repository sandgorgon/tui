package widget

import (
	"testing"
	"time"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/layout"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

// benchFrame reuses roundtrip_test.go's M10 composite frame — a
// realistic mix of the widget catalog rather than one widget in
// isolation, since that's closer to what a real app's View() paints
// every frame.
func benchFrame(theme style.Theme) tui.Node {
	fixedNow := func() time.Time { return time.Unix(0, 0) }
	content := linesNode([]string{"line one", "line two", "line three", "line four"})

	return tui.Box(layout.Vertical,
		tui.Child(layout.Length(1), Tabs([]string{"Inbox", "Sent"}, 0, theme, nil)),
		tui.Child(layout.Length(3), Paragraph("a paragraph long enough to wrap across more than one line", theme.Text())),
		tui.Child(layout.Length(1), ProgressBar(0.35, ProgressBarOptions{Theme: theme})),
		tui.Child(layout.Fill(1), tui.Box(layout.Horizontal,
			tui.Child(layout.Fill(1), List([]string{"apple", "banana", "cherry"}, 1,
				ListOptions{Theme: theme, Selected: []bool{true, false, false}}, nil)),
			tui.Child(layout.Fill(1), Viewport(content, 4)),
		)),
		tui.Child(layout.Length(1), StatusBar(
			[]Segment{{Text: "NORMAL"}}, []Segment{{Text: "status.go"}}, []Segment{{Text: "Ln 1, Col 1"}},
			theme.Text(),
		)),
		tui.Child(layout.Length(1), Spinner(SpinnerOptions{Style: theme.Text(), Now: fixedNow})),
	).Gap(1).Margin(1)
}

// BenchmarkWidgetsReconcileCold measures mounting benchFrame from
// scratch each iteration.
func BenchmarkWidgetsReconcileCold(b *testing.B) {
	theme := style.DefaultDark()
	for b.Loop() {
		var tr tui.Tree
		tr.Reconcile(benchFrame(theme))
	}
}

// BenchmarkWidgetsPaint measures painting an already-reconciled
// composite frame — the steady-state per-redraw cost across the
// M10/M11 widget catalog together.
func BenchmarkWidgetsPaint(b *testing.B) {
	theme := style.DefaultDark()
	var tr tui.Tree
	tr.Reconcile(benchFrame(theme))
	buf := cell.NewBuffer(60, 20)
	p := cell.NewPainter(buf)
	for b.Loop() {
		tr.Paint(p)
	}
}
