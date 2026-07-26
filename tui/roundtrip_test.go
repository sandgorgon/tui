package tui

import (
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/internal/testutil"
	"github.com/sandgorgon/tui/layout"
	"github.com/sandgorgon/tui/render"
	"github.com/sandgorgon/tui/term"
)

// TestAppFrameSurvivesRenderVTRoundTrip runs an App-produced buffer
// through the project's primary rendering-correctness harness (see
// internal/testutil.RoundTrip and docs/DESIGN.md §10): render it to
// ANSI/SGR bytes, parse those bytes back through an independent
// decoder (vt.Parser), and assert the result matches the source
// buffer exactly. Package tui is the first consumer of the harness
// after package render itself (M5) — this is the check the design doc
// says should carry forward into the component model rather than
// being reinvented per package.
func TestAppFrameSurvivesRenderVTRoundTrip(t *testing.T) {
	m := &focusTestModel{}
	a := NewApp(m, 24, 5)
	a.HandleInput(input.KeyEvent{}) // paint a real frame, not just the initial one
	a.Resize(24, 5)

	got := testutil.RoundTrip(a.Buffer(), render.Options{ColorLevel: term.ColorTrueColor})
	if !testutil.BuffersEqual(a.Buffer(), got) {
		t.Errorf("render<->vt round trip mismatch:\n%s", testutil.DiffBuffers(a.Buffer(), got))
	}
}

func TestBoxNestingSurvivesRenderVTRoundTrip(t *testing.T) {
	node := Box(layout.Vertical,
		Child(layout.Length(1), Text("header", cell.Style{Fg: cell.ANSIColor(4), Attr: cell.AttrBold})),
		Child(layout.Fill(1), Box(layout.Horizontal,
			Child(layout.Length(4), Text("side", cell.Style{Fg: cell.ANSIColor(2)})),
			Child(layout.Fill(1), Text("main content here", cell.Style{Underline: cell.UnderlineSingle})),
		)),
	)
	r := reconcile(nil, node)
	buf := cell.NewBuffer(20, 4)
	r.paint(cell.NewPainter(buf))

	got := testutil.RoundTrip(buf, render.Options{ColorLevel: term.ColorTrueColor})
	if !testutil.BuffersEqual(buf, got) {
		t.Errorf("render<->vt round trip mismatch:\n%s", testutil.DiffBuffers(buf, got))
	}
}
