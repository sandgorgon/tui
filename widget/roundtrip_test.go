package widget

import (
	"testing"
	"time"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/internal/testutil"
	"github.com/sandgorgon/tui/layout"
	"github.com/sandgorgon/tui/render"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/term"
	"github.com/sandgorgon/tui/tui"
)

// TestWidgetsSurviveRenderVTRoundTrip runs a frame composed of every
// M10 widget through the project's primary rendering-correctness
// harness (see internal/testutil.RoundTrip and docs/DESIGN.md §10):
// render it to ANSI/SGR bytes, parse those bytes back through an
// independent decoder (vt.Parser), and assert the result matches the
// source buffer exactly — the "golden coverage via the M5 harness"
// docs/DESIGN.md's M10 row calls for.
func TestWidgetsSurviveRenderVTRoundTrip(t *testing.T) {
	theme := style.DefaultDark()
	fixedNow := func() time.Time { return time.Unix(0, 0) }

	content := linesNode([]string{"line one", "line two", "line three", "line four"})

	root := tui.Box(layout.Vertical,
		tui.Child(layout.Length(1), Tabs([]string{"Inbox", "Sent"}, 0, theme, nil)),
		tui.Child(layout.Length(3), Paragraph("a paragraph long enough to wrap across more than one line", theme.Text())),
		tui.Child(layout.Length(1), ProgressBar(0.35, ProgressBarOptions{Theme: theme})),
		tui.Child(layout.Length(1), ProgressBar(0, ProgressBarOptions{Theme: theme, Indeterminate: true, Now: fixedNow})),
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

	buf := cell.NewBuffer(60, 20)
	paintNode(t, root, buf)

	got := testutil.RoundTrip(buf, render.Options{ColorLevel: term.ColorTrueColor})
	if !testutil.BuffersEqual(buf, got) {
		t.Errorf("render<->vt round trip mismatch:\n%s", testutil.DiffBuffers(buf, got))
	}
}
