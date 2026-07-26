package widget

import (
	"os/exec"
	"strings"
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

// TestM11WidgetsSurviveRenderVTRoundTrip is TestWidgetsSurviveRenderVTRoundTrip's
// M11 counterpart: TextInput, TextArea, RadioGroup, CheckboxGroup,
// Tree, Table, and Select composed into one frame and round-tripped
// through the same M5 harness — the "golden coverage" docs/DESIGN.md's
// M11 row calls for. Modal/CommandPalette (which paint via
// OverlayPainter, a different code path — see below) and Terminal
// (real-subprocess timing) get their own round-trip tests instead of
// being folded in here.
func TestM11WidgetsSurviveRenderVTRoundTrip(t *testing.T) {
	theme := style.DefaultDark()

	root := tui.Box(layout.Vertical,
		tui.Child(layout.Length(3), TextInput(TextInputOptions{Theme: theme, Value: "search term", Placeholder: "search"})),
		tui.Child(layout.Length(5), TextArea(TextAreaOptions{Theme: theme, Value: "line one\nline two"})),
		tui.Child(layout.Fill(1), tui.Box(layout.Horizontal,
			tui.Child(layout.Fill(1), RadioGroup([]string{"small", "medium", "large"}, 1, theme, nil)),
			tui.Child(layout.Fill(1), CheckboxGroup([]string{"a", "b", "c"}, []bool{true, false, true}, 0, theme, nil)),
			tui.Child(layout.Fill(1), Tree([]TreeRow{
				{Label: "root", HasChildren: true, Expanded: true},
				{Label: "child", Depth: 1},
			}, 0, theme, nil)),
		)),
		tui.Child(layout.Length(4), Table(testColumns(), [][]string{{"alice", "30"}, {"bob", "25"}}, 0,
			TableOptions{Theme: theme, SortColumn: 0}, nil)),
		tui.Child(layout.Length(3), Select([]string{"red", "green", "blue"}, 1, 1, SelectOptions{Theme: theme, Open: true}, nil)),
	).Gap(1).Margin(1)

	buf := cell.NewBuffer(70, 25)
	paintNode(t, root, buf)

	got := testutil.RoundTrip(buf, render.Options{ColorLevel: term.ColorTrueColor})
	if !testutil.BuffersEqual(buf, got) {
		t.Errorf("render<->vt round trip mismatch:\n%s", testutil.DiffBuffers(buf, got))
	}
}

// TestModalOverlaySurvivesRenderVTRoundTrip covers PaintOverlay, the
// code path Modal/CommandPalette use instead of the ordinary Box-child
// Paint every other widget goes through (see OverlayPainter in package
// tui) — a real App is needed here, since only App.render() invokes
// it.
func TestModalOverlaySurvivesRenderVTRoundTrip(t *testing.T) {
	theme := style.DefaultDark()
	m := &widgetHostModel{node: tui.Box(layout.Horizontal,
		tui.Child(layout.Fill(1), tui.Text("background content", theme.Text())),
		tui.Child(layout.Length(0), Modal(
			tui.Box(layout.Vertical,
				tui.Child(layout.Length(1), tui.Text("Are you sure?", theme.Text())),
			),
			ModalOptions{Theme: theme, Open: true, Title: "Confirm", Width: 24, Height: 5},
		)),
	)}
	app := tui.NewApp(m, 40, 12)

	got := testutil.RoundTrip(app.Buffer(), render.Options{ColorLevel: term.ColorTrueColor})
	if !testutil.BuffersEqual(app.Buffer(), got) {
		t.Errorf("render<->vt round trip mismatch:\n%s", testutil.DiffBuffers(app.Buffer(), got))
	}
}

// TestTerminalOutputSurvivesRenderVTRoundTrip covers Terminal's
// SetRawCell-based blit of a live vt.Screen (see Terminal.Paint) —
// waited for known, stable output first, since a running subprocess's
// timing is otherwise nondeterministic and unsuitable for the other
// static round-trip tests above.
func TestTerminalOutputSurvivesRenderVTRoundTrip(t *testing.T) {
	node := Terminal(TerminalOptions{Command: exec.Command("printf", "round\\ntrip")})
	buf := cell.NewBuffer(20, 4)
	var tr tui.Tree
	tr.Reconcile(node)

	waitFor(t, 2*time.Second, func() { tr.Paint(cell.NewPainter(buf)) }, func() bool {
		return strings.Contains(buf.String(), "round")
	})
	defer tr.Close()

	got := testutil.RoundTrip(buf, render.Options{ColorLevel: term.ColorTrueColor})
	if !testutil.BuffersEqual(buf, got) {
		t.Errorf("render<->vt round trip mismatch:\n%s", testutil.DiffBuffers(buf, got))
	}
}
