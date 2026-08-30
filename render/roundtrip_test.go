package render_test

import (
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/internal/testutil"
	"github.com/sandgorgon/tui/render"
	"github.com/sandgorgon/tui/term"
	"github.com/sandgorgon/tui/vt"
)

// These are the M5 deliverable per docs/DESIGN.md §10: the render<->vt
// round trip. Because vt.Parser is an independent decoder of
// render.Renderer's own encoder, a passing round trip is strong
// evidence the renderer emits well-formed sequences that mean what was
// intended — not just a render package test, but the correctness
// oracle the rest of the project reuses from M6 on.

func assertRoundTrips(t *testing.T, name string, back *cell.Buffer, opts render.Options) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		got := testutil.RoundTrip(back, opts)
		if !testutil.BuffersEqual(back, got) {
			t.Errorf("round trip mismatch:\n%s", testutil.DiffBuffers(back, got))
		}
	})
}

func TestRoundTripPlainText(t *testing.T) {
	buf := cell.NewBuffer(10, 2)
	for i, r := range "hello" {
		buf.Set(i, 0, cell.Cell{Rune: r, Width: 1})
	}
	for i, r := range "world!" {
		buf.Set(i, 1, cell.Cell{Rune: r, Width: 1})
	}
	assertRoundTrips(t, "plain", buf, render.Options{ColorLevel: term.ColorTrueColor})
}

func TestRoundTripTruecolor(t *testing.T) {
	buf := cell.NewBuffer(5, 1)
	buf.Set(0, 0, cell.Cell{Rune: 'r', Width: 1, Style: cell.Style{Fg: cell.RGBColor(255, 0, 0), Bg: cell.RGBColor(0, 0, 128)}})
	buf.Set(1, 0, cell.Cell{Rune: 'g', Width: 1, Style: cell.Style{Fg: cell.RGBColor(0, 255, 0)}})
	buf.Set(2, 0, cell.Cell{Rune: 'b', Width: 1, Style: cell.Style{Fg: cell.RGBColor(0, 0, 255)}})
	assertRoundTrips(t, "truecolor", buf, render.Options{ColorLevel: term.ColorTrueColor})
}

func TestRoundTripANSIAndIndexedColor(t *testing.T) {
	buf := cell.NewBuffer(5, 1)
	buf.Set(0, 0, cell.Cell{Rune: 'a', Width: 1, Style: cell.Style{Fg: cell.ANSIColor(3), Bg: cell.ANSIColor(12)}})
	buf.Set(1, 0, cell.Cell{Rune: 'b', Width: 1, Style: cell.Style{Fg: cell.IndexedColor(200)}})
	assertRoundTrips(t, "ansi+indexed", buf, render.Options{ColorLevel: term.ColorTrueColor})
}

func TestRoundTripAttributes(t *testing.T) {
	buf := cell.NewBuffer(8, 1)
	buf.Set(0, 0, cell.Cell{Rune: 'a', Width: 1, Style: cell.Style{Attr: cell.AttrBold}})
	buf.Set(1, 0, cell.Cell{Rune: 'b', Width: 1, Style: cell.Style{Attr: cell.AttrItalic | cell.AttrReverse}})
	buf.Set(2, 0, cell.Cell{Rune: 'c', Width: 1, Style: cell.Style{Attr: cell.AttrStrikethrough | cell.AttrBlink}})
	buf.Set(3, 0, cell.Cell{Rune: 'd', Width: 1, Style: cell.Style{Underline: cell.UnderlineCurly}})
	buf.Set(4, 0, cell.Cell{Rune: 'e', Width: 1, Style: cell.Style{Underline: cell.UnderlineDouble, UnderlineColor: cell.RGBColor(10, 20, 30)}})
	assertRoundTrips(t, "attributes", buf, render.Options{ColorLevel: term.ColorTrueColor})
}

func TestRoundTripWideRunes(t *testing.T) {
	buf := cell.NewBuffer(6, 1)
	buf.Set(0, 0, cell.Cell{Rune: 'a', Width: 1})
	buf.Set(1, 0, cell.Cell{Rune: '中', Width: 2, Style: cell.Style{Fg: cell.ANSIColor(2)}})
	buf.Set(2, 0, cell.Cell{Width: 0, Style: cell.Style{Fg: cell.ANSIColor(2)}})
	buf.Set(3, 0, cell.Cell{Rune: 'b', Width: 1})
	buf.Set(4, 0, cell.Cell{Rune: '文', Width: 2})
	buf.Set(5, 0, cell.Cell{Width: 0})
	assertRoundTrips(t, "wide-runes", buf, render.Options{ColorLevel: term.ColorTrueColor})
}

func TestRoundTripHyperlink(t *testing.T) {
	buf := cell.NewBuffer(10, 1)
	style := cell.Style{Hyperlink: "https://example.com", Underline: cell.UnderlineSingle}
	for i, r := range "click" {
		buf.Set(i, 0, cell.Cell{Rune: r, Width: 1, Style: style})
	}
	for i, r := range " plain" {
		buf.Set(5+i, 0, cell.Cell{Rune: r, Width: 1})
	}
	assertRoundTrips(t, "hyperlink", buf, render.Options{ColorLevel: term.ColorTrueColor})
}

func TestRoundTripKitchenSink(t *testing.T) {
	// A styled frame combining everything at once: colors, attributes,
	// wide runes, and a hyperlink, across multiple rows.
	buf := cell.NewBuffer(20, 3)
	header := cell.Style{Attr: cell.AttrBold | cell.AttrReverse, Fg: cell.RGBColor(255, 255, 255), Bg: cell.RGBColor(30, 30, 120)}
	for i, r := range " STATUS: 中文 OK    " {
		buf.Set(i, 0, cell.Cell{Rune: r, Width: 1, Style: header})
	}
	buf.Set(9, 0, cell.Cell{Rune: '中', Width: 2, Style: header})
	buf.Set(10, 0, cell.Cell{Width: 0, Style: header})
	buf.Set(11, 0, cell.Cell{Rune: '文', Width: 2, Style: header})
	buf.Set(12, 0, cell.Cell{Width: 0, Style: header})

	linkStyle := cell.Style{Hyperlink: "https://example.com/docs", Fg: cell.IndexedColor(33), Underline: cell.UnderlineSingle}
	for i, r := range "see docs" {
		buf.Set(i, 1, cell.Cell{Rune: r, Width: 1, Style: linkStyle})
	}

	plain := cell.Style{}
	for i, r := range "ready." {
		buf.Set(i, 2, cell.Cell{Rune: r, Width: 1, Style: plain})
	}

	assertRoundTrips(t, "kitchen-sink", buf, render.Options{ColorLevel: term.ColorTrueColor})
}

// TestRoundTripDownsampledColorsAreInternallyConsistent verifies the
// round trip at reduced color levels. This can't assert equality to
// the *original* buffer — downsampling is lossy by definition — but it
// proves the pipeline is internally consistent: whatever color the
// renderer decided to downsample to is exactly what the parser reads
// back, by comparing against a buffer already expressed in the
// downsampled color the encoder is expected to choose.
func TestRoundTripDownsampledColorsAreInternallyConsistent(t *testing.T) {
	back := cell.NewBuffer(3, 1)
	back.Set(0, 0, cell.Cell{Rune: 'r', Width: 1, Style: cell.Style{Fg: cell.RGBColor(255, 0, 0)}}) // -> bright red (16-color)

	got := testutil.RoundTrip(back, render.Options{ColorLevel: term.Color16})

	want := cell.NewBuffer(3, 1)
	want.Set(0, 0, cell.Cell{Rune: 'r', Width: 1, Style: cell.Style{Fg: cell.ANSIColor(9)}})
	if !testutil.BuffersEqual(want, got) {
		t.Errorf("round trip at Color16 mismatch:\n%s", testutil.DiffBuffers(want, got))
	}
}

// TestRoundTripIncrementalFrames drives a persistent Renderer and
// Screen across several frames (not a fresh pair per call, unlike
// testutil.RoundTrip), exercising incremental diff output — cursor
// tracking and style-transition state carried across calls — rather
// than only ever a single full-screen paint.
func TestRoundTripIncrementalFrames(t *testing.T) {
	r := render.NewRenderer(render.Options{ColorLevel: term.ColorTrueColor})
	screen := vt.NewScreen(10, 2)
	parser := vt.NewParser()

	frame := func(back *cell.Buffer) {
		t.Helper()
		var buf writeBuf
		if err := r.Render(&buf, back, 0, 0, true); err != nil {
			t.Fatal(err)
		}
		parser.Feed(buf.b, screen)
		if !testutil.BuffersEqual(back, screen.Buffer()) {
			t.Fatalf("incremental round trip mismatch:\n%s", testutil.DiffBuffers(back, screen.Buffer()))
		}
	}

	f1 := cell.NewBuffer(10, 2)
	for i, r := range "hello" {
		f1.Set(i, 0, cell.Cell{Rune: r, Width: 1, Style: cell.Style{Fg: cell.ANSIColor(2)}})
	}
	frame(f1)

	// Change one cell's rune and style; leave everything else, including
	// the untouched second row, exactly as-is.
	f2 := cell.NewBuffer(10, 2)
	for i, r := range "hello" {
		f2.Set(i, 0, cell.Cell{Rune: r, Width: 1, Style: cell.Style{Fg: cell.ANSIColor(2)}})
	}
	f2.Set(1, 0, cell.Cell{Rune: 'a', Width: 1, Style: cell.Style{Attr: cell.AttrBold, Fg: cell.RGBColor(1, 2, 3)}})
	frame(f2)

	// A change on a different row, plus clearing part of row 0.
	f3 := cell.NewBuffer(10, 2)
	f3.Set(0, 0, cell.Cell{Rune: 'h', Width: 1, Style: cell.Style{Fg: cell.ANSIColor(2)}})
	for i, r := range "world" {
		f3.Set(i, 1, cell.Cell{Rune: r, Width: 1, Style: cell.Style{Hyperlink: "https://x"}})
	}
	frame(f3)
}

// TestRoundTripResizeErasesStaleContent models a real terminal resize:
// unlike testutil.RoundTrip (a fresh vt.Screen every call, which can
// never show stale content from a size it never was) or
// TestRoundTripIncrementalFrames (never changes size), it resizes the
// same persistent Screen with vt.Screen.Resize — which, matching real
// terminal behavior, preserves existing content rather than clearing
// it. That's what exposed the real bug this guards: Renderer.Render
// resetting its own bookkeeping of "what the terminal shows" to blank
// on a size change without telling the real terminal to actually erase
// anything, so a cell blank in both the new frame and the reset
// bookkeeping was wrongly assumed already correct and left whatever a
// differently-sized previous frame had painted there.
func TestRoundTripResizeErasesStaleContent(t *testing.T) {
	r := render.NewRenderer(render.Options{ColorLevel: term.ColorTrueColor})
	screen := vt.NewScreen(10, 2)
	parser := vt.NewParser()

	f1 := cell.NewBuffer(10, 2)
	for y := range 2 {
		for x := range 10 {
			f1.Set(x, y, cell.Cell{Rune: 'X', Width: 1, Style: cell.Style{Fg: cell.ANSIColor(1)}})
		}
	}
	var buf1 writeBuf
	if err := r.Render(&buf1, f1, 0, 0, true); err != nil {
		t.Fatal(err)
	}
	parser.Feed(buf1.b, screen)

	// Shrink, the way a real terminal resize would: existing content is
	// preserved, not cleared.
	screen.Resize(6, 2)

	// The new frame is mostly blank at this size (as an app's own
	// buffer commonly is: margins, gaps, a shorter line than before).
	f2 := cell.NewBuffer(6, 2)
	f2.Set(0, 0, cell.Cell{Rune: 'h', Width: 1})
	f2.Set(1, 0, cell.Cell{Rune: 'i', Width: 1})

	var buf2 writeBuf
	if err := r.Render(&buf2, f2, 0, 0, true); err != nil {
		t.Fatal(err)
	}
	parser.Feed(buf2.b, screen)

	if !testutil.BuffersEqual(f2, screen.Buffer()) {
		t.Errorf("stale content survived the resize:\n%s", testutil.DiffBuffers(f2, screen.Buffer()))
	}
}

type writeBuf struct{ b []byte }

func (w *writeBuf) Write(p []byte) (int, error) {
	w.b = append(w.b, p...)
	return len(p), nil
}
