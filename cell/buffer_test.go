package cell

import "testing"

func TestNewBufferIsBlank(t *testing.T) {
	b := NewBuffer(3, 2)
	if b.Width != 3 || b.Height != 2 {
		t.Fatalf("dims = %dx%d, want 3x2", b.Width, b.Height)
	}
	for y := range 2 {
		for x := range 3 {
			if got := b.At(x, y); got != Blank {
				t.Errorf("At(%d,%d) = %+v, want Blank", x, y, got)
			}
		}
	}
}

func TestSetAndAtOutOfBounds(t *testing.T) {
	b := NewBuffer(2, 2)
	b.Set(5, 5, Cell{Rune: 'x', Width: 1}) // out of bounds: silently ignored
	for y := range 2 {
		for x := range 2 {
			if got := b.At(x, y); got != Blank {
				t.Errorf("out-of-bounds Set leaked into At(%d,%d) = %+v", x, y, got)
			}
		}
	}
	if got := b.At(-1, 0); got != (Cell{}) {
		t.Errorf("At(-1,0) = %+v, want zero Cell", got)
	}
	if got := b.At(0, 5); got != (Cell{}) {
		t.Errorf("At(0,5) = %+v, want zero Cell", got)
	}
}

func TestSetAndAt(t *testing.T) {
	b := NewBuffer(3, 3)
	c := Cell{Rune: 'x', Style: Style{Fg: RGBColor(1, 2, 3)}, Width: 1}
	b.Set(1, 1, c)
	if got := b.At(1, 1); got != c {
		t.Errorf("At(1,1) = %+v, want %+v", got, c)
	}
	if got := b.At(0, 0); got != Blank {
		t.Errorf("At(0,0) = %+v, want untouched Blank", got)
	}
}

func TestResizeDiscardsContent(t *testing.T) {
	b := NewBuffer(2, 2)
	b.Set(0, 0, Cell{Rune: 'x', Width: 1})
	b.Resize(4, 1)
	if b.Width != 4 || b.Height != 1 {
		t.Fatalf("dims after resize = %dx%d, want 4x1", b.Width, b.Height)
	}
	if got := b.At(0, 0); got != Blank {
		t.Errorf("At(0,0) after resize = %+v, want Blank (content discarded)", got)
	}
}

func TestClearWithStyle(t *testing.T) {
	b := NewBuffer(2, 2)
	style := Style{Fg: ANSIColor(2)}
	b.Clear(style)
	want := Cell{Rune: ' ', Style: style, Width: 1}
	if got := b.At(0, 0); got != want {
		t.Errorf("At(0,0) after Clear = %+v, want %+v", got, want)
	}
}

func TestFillClipsToBounds(t *testing.T) {
	b := NewBuffer(4, 4)
	c := Cell{Rune: '#', Width: 1}
	b.Fill(-2, -2, 5, 5, c) // extends past top-left and into the middle

	for y := range 4 {
		for x := range 4 {
			inFilled := x < 3 && y < 3 // [-2,-2)-(3,3) clipped to [0,0)-(3,3)
			got := b.At(x, y)
			if inFilled && got != c {
				t.Errorf("At(%d,%d) = %+v, want filled %+v", x, y, got, c)
			}
			if !inFilled && got != Blank {
				t.Errorf("At(%d,%d) = %+v, want untouched Blank", x, y, got)
			}
		}
	}
}

func TestBufferString(t *testing.T) {
	b := NewBuffer(4, 2)
	b.Set(0, 0, Cell{Rune: 'h', Width: 1})
	b.Set(1, 0, Cell{Rune: 'i', Width: 1})
	// A wide rune at (2,0) plus its continuation at (3,0).
	b.Set(2, 0, Cell{Rune: '中', Width: 2})
	b.Set(3, 0, Cell{Width: 0})

	want := "hi中\n    "
	if got := b.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
