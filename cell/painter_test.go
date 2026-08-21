package cell

import "testing"

func TestPainterSetCellBasic(t *testing.T) {
	b := NewBuffer(3, 3)
	p := NewPainter(b)
	style := Style{Fg: ANSIColor(1)}
	p.SetCell(1, 1, 'x', style)

	want := Cell{Rune: 'x', Style: style, Width: 1}
	if got := b.At(1, 1); got != want {
		t.Errorf("At(1,1) = %+v, want %+v", got, want)
	}
}

func TestPainterSetCellOutOfBounds(t *testing.T) {
	b := NewBuffer(2, 2)
	p := NewPainter(b)
	p.SetCell(-1, 0, 'x', Style{})
	p.SetCell(0, -1, 'x', Style{})
	p.SetCell(2, 0, 'x', Style{})
	p.SetCell(0, 2, 'x', Style{})

	for y := range 2 {
		for x := range 2 {
			if got := b.At(x, y); got != Blank {
				t.Errorf("out-of-bounds SetCell leaked into At(%d,%d) = %+v", x, y, got)
			}
		}
	}
}

func TestPainterWideRune(t *testing.T) {
	b := NewBuffer(4, 1)
	p := NewPainter(b)
	style := Style{Fg: ANSIColor(3)}
	p.SetCell(1, 0, '中', style)

	wantPrimary := Cell{Rune: '中', Style: style, Width: 2}
	if got := b.At(1, 0); got != wantPrimary {
		t.Errorf("At(1,0) = %+v, want %+v", got, wantPrimary)
	}
	wantCont := Cell{Style: style, Width: 0}
	if got := b.At(2, 0); got != wantCont {
		t.Errorf("At(2,0) (continuation) = %+v, want %+v", got, wantCont)
	}
	if !b.At(2, 0).IsContinuation() {
		t.Error("At(2,0).IsContinuation() = false, want true")
	}
}

func TestPainterWideRuneNotDrawnWhenItDoesntFit(t *testing.T) {
	b := NewBuffer(3, 1)
	p := NewPainter(b)
	// Only column 2 remains (last column); a wide rune needs 2 columns,
	// so it must not be drawn at all, not truncated.
	p.SetCell(2, 0, '中', Style{})

	if got := b.At(2, 0); got != Blank {
		t.Errorf("At(2,0) = %+v, want untouched Blank (wide rune shouldn't fit)", got)
	}
}

func TestPainterClipTranslatesCoordinates(t *testing.T) {
	b := NewBuffer(5, 5)
	root := NewPainter(b)
	child := root.Clip(Rect{X: 2, Y: 2, W: 2, H: 2})

	child.SetCell(0, 0, 'a', Style{})

	if got := b.At(2, 2); got.Rune != 'a' {
		t.Errorf("At(2,2) = %+v, want Rune 'a' (child (0,0) should map to absolute (2,2))", got)
	}
}

func TestPainterClipIntersectsParent(t *testing.T) {
	b := NewBuffer(4, 4)
	root := NewPainter(b).Clip(Rect{X: 1, Y: 1, W: 2, H: 2}) // absolute [1,1)-(3,3)

	// A child requesting a rect that extends beyond the parent's clip
	// must be constrained to the intersection.
	child := root.Clip(Rect{X: 0, Y: 0, W: 10, H: 10})
	w, h := child.Size()
	if w != 2 || h != 2 {
		t.Fatalf("child.Size() = %dx%d, want 2x2 (intersected with parent)", w, h)
	}

	// Drawing at a local coordinate outside the intersected clip must
	// be a no-op even though it would be inside the *requested* rect.
	child.SetCell(3, 3, 'z', Style{})
	for y := range 4 {
		for x := range 4 {
			if got := b.At(x, y); got != Blank {
				t.Errorf("At(%d,%d) = %+v, want untouched Blank", x, y, got)
			}
		}
	}
}

func TestPainterText(t *testing.T) {
	b := NewBuffer(6, 1)
	p := NewPainter(b)
	style := Style{Attr: AttrBold}

	n := p.Text(0, 0, "a中b", style)
	if n != 4 { // 'a'(1) + '中'(2) + 'b'(1)
		t.Errorf("Text advanced %d columns, want 4", n)
	}

	want := "a中b  "
	if got := b.String(); got != want {
		t.Errorf("buffer after Text = %q, want %q", got, want)
	}
	if got := b.At(0, 0); got.Style != style {
		t.Errorf("style not applied: At(0,0).Style = %+v, want %+v", got.Style, style)
	}
}

func TestPainterSetCellControlRuneSubstitutesPlaceholder(t *testing.T) {
	b := NewBuffer(3, 1)
	p := NewPainter(b)
	style := Style{Attr: AttrBold}
	p.SetCell(1, 0, '\t', style)

	want := Cell{Rune: ' ', Style: style, Width: 1}
	if got := b.At(1, 0); got != want {
		t.Errorf("At(1,0) = %+v, want %+v (a control rune must become a printable placeholder, not be stored verbatim — otherwise the emitted byte desyncs a real terminal's cursor from the renderer's own x+=Width bookkeeping)", got, want)
	}
}

func TestPainterFillControlRuneSubstitutesPlaceholder(t *testing.T) {
	b := NewBuffer(3, 1)
	p := NewPainter(b)
	p.Fill(0, 0, 3, 1, '\x01', Style{})

	for x := range 3 {
		if got := b.At(x, 0); got.Rune != ' ' || got.Width != 1 {
			t.Errorf("At(%d,0) = %+v, want a space placeholder", x, got)
		}
	}
}

func TestPainterSetCellZeroWidthRuneKeepsRune(t *testing.T) {
	b := NewBuffer(3, 1)
	p := NewPainter(b)
	const combiningAcute = '\u0301' // zero-width combining mark, not a control rune
	p.SetCell(1, 0, combiningAcute, Style{})

	want := Cell{Rune: combiningAcute, Width: 1}
	if got := b.At(1, 0); got != want {
		t.Errorf("At(1,0) = %+v, want %+v (zero-width combining marks keep their own rune, per Painter's documented combining-mark simplification — only true control runes get substituted)", got, want)
	}
}

func TestPainterSetRawCellWritesVerbatim(t *testing.T) {
	b := NewBuffer(3, 1)
	p := NewPainter(b)

	// A continuation cell (Rune 0, Width 0) written via SetCell would
	// be reinterpreted (wcwidth.RuneWidth(0) doesn't mean "skip me");
	// SetRawCell must store it exactly as given.
	cont := Cell{Style: Style{Attr: AttrBold}, Width: 0}
	p.SetRawCell(1, 0, cont)

	if got := b.At(1, 0); got != cont {
		t.Errorf("At(1,0) = %+v, want %+v (verbatim)", got, cont)
	}
}

func TestPainterSetRawCellClipsAndTranslates(t *testing.T) {
	b := NewBuffer(5, 5)
	child := NewPainter(b).Clip(Rect{X: 2, Y: 2, W: 2, H: 2})

	c := Cell{Rune: 'z', Width: 1}
	child.SetRawCell(0, 0, c)
	if got := b.At(2, 2); got != c {
		t.Errorf("At(2,2) = %+v, want %+v (child (0,0) maps to absolute (2,2))", got, c)
	}

	child.SetRawCell(5, 5, c) // outside the clip entirely
	if got := b.At(4, 4); got == c {
		t.Error("out-of-clip SetRawCell should not have written anywhere")
	}
}

func TestPainterFillClipsToPainterBounds(t *testing.T) {
	b := NewBuffer(4, 4)
	p := NewPainter(b).Clip(Rect{X: 1, Y: 1, W: 2, H: 2})

	p.Fill(-5, -5, 100, 100, '#', Style{})

	for y := range 4 {
		for x := range 4 {
			inFilled := x >= 1 && x < 3 && y >= 1 && y < 3
			got := b.At(x, y)
			if inFilled && got.Rune != '#' {
				t.Errorf("At(%d,%d) = %+v, want filled with '#'", x, y, got)
			}
			if !inFilled && got != Blank {
				t.Errorf("At(%d,%d) = %+v, want untouched Blank (outside painter's clip)", x, y, got)
			}
		}
	}
}
