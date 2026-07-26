package widget

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/internal/testutil"
	"github.com/sandgorgon/tui/layout"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

// TestGoldenWidgets is the "full golden-file coverage across all
// widgets" M12 item (docs/DESIGN.md §8/§10): one representative,
// deterministic frame per widget.Xxx constructor, dumped as a text
// grid (internal/testutil.Golden -> cell.Buffer.String) and diffed
// against a checked-in fixture under testdata/golden/. This protects
// a widget's actual painted *content/shape* against silent regression
// — a thing the render<->vt round-trip tests (roundtrip_test.go)
// deliberately don't check, since a round trip only proves whatever
// was painted survives re-encoding, not that it's still the right
// thing to paint.
//
// Widgets with nondeterministic or App-dependent painting (Modal,
// CommandPalette via OverlayPainter; Terminal via a real subprocess)
// get their own case below using the same host patterns already
// established in modal_test.go/commandpalette_test.go/terminal_test.go,
// rather than paintNode.
func TestGoldenWidgets(t *testing.T) {
	theme := style.DefaultDark()
	fixedNow := func() time.Time { return time.Unix(0, 0) }

	cases := []struct {
		name string
		w, h int
		node tui.Node
	}{
		{"paragraph", 24, 3, Paragraph("a paragraph long enough to wrap across more than one line", theme.Text())},
		{"list", 12, 4, List([]string{"apple", "banana", "cherry"}, 1, ListOptions{Theme: theme, Selected: []bool{true, false, false}}, nil)},
		{"viewport", 12, 3, Viewport(linesNode([]string{"line one", "line two", "line three", "line four"}), 4)},
		{"tabs", 20, 1, Tabs([]string{"Inbox", "Sent", "Trash"}, 1, theme, nil)},
		{"statusbar", 30, 1, StatusBar(
			[]Segment{{Text: "NORMAL"}}, []Segment{{Text: "status.go"}}, []Segment{{Text: "Ln 1, Col 1"}},
			theme.Text(),
		)},
		{"progressbar", 20, 1, ProgressBar(0.35, ProgressBarOptions{Theme: theme})},
		{"spinner", 1, 1, Spinner(SpinnerOptions{Style: theme.Text(), Now: fixedNow})},
		{"textinput", 20, 3, TextInput(TextInputOptions{Theme: theme, Value: "search term", Placeholder: "search"})},
		{"textarea", 20, 5, TextArea(TextAreaOptions{Theme: theme, Value: "line one\nline two"})},
		{"radiogroup", 12, 3, RadioGroup([]string{"small", "medium", "large"}, 1, theme, nil)},
		{"checkboxgroup", 8, 3, CheckboxGroup([]string{"a", "b", "c"}, []bool{true, false, true}, 0, theme, nil)},
		{"select-closed", 12, 1, Select([]string{"red", "green", "blue"}, 1, 1, SelectOptions{Theme: theme}, nil)},
		{"select-open", 12, 4, Select([]string{"red", "green", "blue"}, 1, 1, SelectOptions{Theme: theme, Open: true}, nil)},
		{"tree", 16, 2, Tree([]TreeRow{
			{Label: "root", HasChildren: true, Expanded: true},
			{Label: "child", Depth: 1},
		}, 0, theme, nil)},
		{"table", 12, 3, Table(testColumns(), [][]string{{"alice", "30"}, {"bob", "25"}}, 0,
			TableOptions{Theme: theme, SortColumn: 0}, nil)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := cell.NewBuffer(tc.w, tc.h)
			paintNode(t, tc.node, buf)
			testutil.Golden(t, "widget_"+tc.name, buf)
		})
	}
}

func TestGoldenModal(t *testing.T) {
	m := &widgetHostModel{node: tui.Box(layout.Horizontal,
		tui.Child(layout.Fill(1), tui.Text("background content", cell.Style{})),
		tui.Child(layout.Length(0), Modal(
			Field1AndField2(),
			ModalOptions{Theme: style.DefaultDark(), Open: true, Title: "Confirm", Width: 24, Height: 8},
		)),
	)}
	app := tui.NewApp(m, 40, 12)
	testutil.Golden(t, "widget_modal", app.Buffer())
}

func TestGoldenCommandPalette(t *testing.T) {
	m := &paletteHostModel{open: true}
	app := tui.NewApp(m, 40, 10)
	testutil.Golden(t, "widget_commandpalette", app.Buffer())
}

func TestGoldenTerminal(t *testing.T) {
	node := Terminal(TerminalOptions{Command: exec.Command("printf", "hello")})
	buf := cell.NewBuffer(20, 3)
	var tr tui.Tree
	tr.Reconcile(node)

	waitFor(t, 2*time.Second, func() { tr.Paint(cell.NewPainter(buf)) }, func() bool {
		return strings.Contains(buf.String(), "hello")
	})
	defer tr.Close()

	testutil.Golden(t, "widget_terminal", buf)
}
