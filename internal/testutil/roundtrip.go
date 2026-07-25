package testutil

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/render"
	"github.com/sandgorgon/tui/vt"
)

// RoundTrip is the project's primary rendering-correctness harness
// (see docs/DESIGN.md §3.2 and §10): it renders back through a fresh
// render.Renderer configured with opts, feeds the resulting bytes
// through a fresh vt.Parser into a vt.Screen sized to match, and
// returns that Screen's resulting buffer. Because vt.Parser is an
// independent decoder of render.Renderer's own encoder, a caller
// asserting RoundTrip's result equals back (see BuffersEqual) is
// strong evidence the renderer emits well-formed sequences that mean
// what was intended — this is reused from M6 on for the component
// model and every widget batch, not just package render's own tests.
func RoundTrip(back *cell.Buffer, opts render.Options) *cell.Buffer {
	r := render.NewRenderer(opts)
	var buf bytes.Buffer
	_ = r.Render(&buf, back, 0, 0, false)

	s := vt.NewScreen(back.Width, back.Height)
	vt.NewParser().Feed(buf.Bytes(), s)
	return s.Buffer()
}

// BuffersEqual reports whether a and b have the same dimensions and
// identical cells (rune, style, and width) at every position.
func BuffersEqual(a, b *cell.Buffer) bool {
	if a.Width != b.Width || a.Height != b.Height {
		return false
	}
	for y := range a.Height {
		for x := range a.Width {
			if a.At(x, y) != b.At(x, y) {
				return false
			}
		}
	}
	return true
}

// DiffBuffers returns a human-readable description of the first few
// cells where want and got differ, for test failure messages.
func DiffBuffers(want, got *cell.Buffer) string {
	if want.Width != got.Width || want.Height != got.Height {
		return fmt.Sprintf("size mismatch: want %dx%d, got %dx%d", want.Width, want.Height, got.Width, got.Height)
	}
	var sb strings.Builder
	n := 0
	for y := range want.Height {
		for x := range want.Width {
			w, g := want.At(x, y), got.At(x, y)
			if w != g {
				fmt.Fprintf(&sb, "(%d,%d): want %+v, got %+v\n", x, y, w, g)
				n++
				if n >= 10 {
					sb.WriteString("... (more differences omitted)\n")
					return sb.String()
				}
			}
		}
	}
	return sb.String()
}
