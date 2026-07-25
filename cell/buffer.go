package cell

import "strings"

// Buffer is a fixed-size grid of Cells.
type Buffer struct {
	Width, Height int
	cells         []Cell
}

// NewBuffer returns a Buffer of the given size, filled with Blank.
func NewBuffer(width, height int) *Buffer {
	b := &Buffer{}
	b.Resize(width, height)
	return b
}

// Resize changes the buffer's dimensions in place, discarding all
// content and refilling with Blank. Widgets that need to preserve
// content across a resize (e.g. scrollback) must do so themselves —
// Buffer has no opinion about that.
func (b *Buffer) Resize(width, height int) {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	b.Width, b.Height = width, height
	b.cells = make([]Cell, width*height)
	b.Clear(Style{})
}

func (b *Buffer) inBounds(x, y int) bool {
	return x >= 0 && x < b.Width && y >= 0 && y < b.Height
}

// At returns the cell at (x,y), or the zero Cell if out of bounds.
func (b *Buffer) At(x, y int) Cell {
	if !b.inBounds(x, y) {
		return Cell{}
	}
	return b.cells[y*b.Width+x]
}

// Set stores c at (x,y) verbatim; out-of-bounds writes are silently
// ignored. Set performs no wide-rune bookkeeping — see Painter for the
// ergonomic, width-aware way to draw text.
func (b *Buffer) Set(x, y int, c Cell) {
	if !b.inBounds(x, y) {
		return
	}
	b.cells[y*b.Width+x] = c
}

// Clear fills the entire buffer with a blank cell in the given style.
func (b *Buffer) Clear(style Style) {
	blank := Blank
	blank.Style = style
	for i := range b.cells {
		b.cells[i] = blank
	}
}

// Fill sets every cell in the rectangle [x,y)-[x+w,y+h) to c, clipped
// to the buffer's bounds.
func (b *Buffer) Fill(x, y, w, h int, c Cell) {
	x0, y0 := max(x, 0), max(y, 0)
	x1, y1 := min(x+w, b.Width), min(y+h, b.Height)
	for row := y0; row < y1; row++ {
		for col := x0; col < x1; col++ {
			b.cells[row*b.Width+col] = c
		}
	}
}

// String dumps the buffer as rows of runes separated by newlines,
// ignoring style and skipping continuation cells (a wide rune's second
// half). Intended for golden-buffer tests (see docs/DESIGN.md §10), not
// production rendering — that's package render's job.
func (b *Buffer) String() string {
	var sb strings.Builder
	for y := 0; y < b.Height; y++ {
		if y > 0 {
			sb.WriteByte('\n')
		}
		for x := 0; x < b.Width; x++ {
			c := b.At(x, y)
			if c.IsContinuation() {
				continue
			}
			r := c.Rune
			if r == 0 {
				r = ' '
			}
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
