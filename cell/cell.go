package cell

// Cell is a single terminal grid position: a rune, its style, and its
// display width.
//
// A rune wider than one column (see internal/wcwidth) occupies two
// adjacent Cells: the first holds the rune with Width 2, and the
// second is a continuation cell (Width 0, Rune 0) that a renderer must
// skip — the wide glyph visually occupies both columns, so nothing
// separate is drawn for the continuation cell. Buffer.Set stores
// exactly what it's given with no such bookkeeping; Painter is what
// maintains this invariant when drawing text (see painter.go).
type Cell struct {
	Rune  rune
	Style Style
	Width uint8
}

// Blank is the zero-value-equivalent "empty" cell: a single space with
// default style. It's what Buffer.Clear fills with by default.
var Blank = Cell{Rune: ' ', Width: 1}

// IsContinuation reports whether c is the second half of a wide rune
// occupying the previous cell, and so should be skipped when rendering.
func (c Cell) IsContinuation() bool { return c.Width == 0 }
