package render

import (
	"io"
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/term"
)

// fillBenchBuffer paints a mix of plain and colored/attributed text
// across every row, representative of a real widget-heavy frame
// rather than a blank buffer (which would make the diff/SGR paths
// trivially cheap).
func fillBenchBuffer(w, h int) *cell.Buffer {
	buf := cell.NewBuffer(w, h)
	styles := []cell.Style{
		{},
		{Fg: cell.ANSIColor(2), Attr: cell.AttrBold},
		{Fg: cell.RGBColor(200, 120, 40), Bg: cell.RGBColor(20, 20, 20)},
		{Attr: cell.AttrReverse, Underline: cell.UnderlineSingle},
	}
	text := "the quick brown fox jumps over the lazy dog 0123456789 "
	for y := range h {
		style := styles[y%len(styles)]
		for x := range w {
			r := rune(text[(x+y)%len(text)])
			buf.Set(x, y, cell.Cell{Rune: r, Width: 1, Style: style})
		}
	}
	return buf
}

// BenchmarkRenderFullFrame measures a Renderer's first Render call
// against a given frame size, which always does a full repaint (no
// prior frame to diff against) — the diff/SGR-writer cost floor.
func BenchmarkRenderFullFrame(b *testing.B) {
	buf := fillBenchBuffer(120, 40)
	for b.Loop() {
		r := NewRenderer(Options{ColorLevel: term.ColorTrueColor})
		_ = r.Render(io.Discard, buf, 0, 0, true)
	}
}

// BenchmarkRenderNoOpDiff measures repeated Render calls against an
// unchanged frame: still an O(width*height) cell-by-cell scan (no
// early-out on an untouched buffer), but renderRow's diff finds
// nothing to emit, so this should track well below
// BenchmarkRenderFullFrame — a regression toward that cost means the
// diff is doing unnecessary emission work, not just the unavoidable
// scan.
func BenchmarkRenderNoOpDiff(b *testing.B) {
	buf := fillBenchBuffer(120, 40)
	r := NewRenderer(Options{ColorLevel: term.ColorTrueColor})
	_ = r.Render(io.Discard, buf, 0, 0, true)

	for b.Loop() {
		_ = r.Render(io.Discard, buf, 0, 0, true)
	}
}

// BenchmarkRenderSmallDiff measures Render against a frame where only
// a single row changed between calls — the common case for an
// interactive app redrawing after one keystroke.
func BenchmarkRenderSmallDiff(b *testing.B) {
	buf := fillBenchBuffer(120, 40)
	r := NewRenderer(Options{ColorLevel: term.ColorTrueColor})
	_ = r.Render(io.Discard, buf, 0, 0, true)

	toggle := false
	for b.Loop() {
		toggle = !toggle
		r2 := ' '
		if toggle {
			r2 = 'x'
		}
		buf.Set(10, 20, cell.Cell{Rune: r2, Width: 1})
		_ = r.Render(io.Discard, buf, 0, 0, true)
	}
}
