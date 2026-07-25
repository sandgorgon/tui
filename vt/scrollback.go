package vt

import "github.com/sandgorgon/tui/cell"

// defaultScrollbackLines is how many lines newScreen keeps by default.
const defaultScrollbackLines = 10000

// scrollback is a bounded ring buffer of rows scrolled off the top of
// the primary screen. Lines pushed beyond its capacity are discarded,
// oldest first. The alternate screen never contributes to it (see
// Screen.scrollRegionUp), matching real terminal behavior — full-screen
// apps like vim/htop/less don't pollute your scrollback.
type scrollback struct {
	lines []([]cell.Cell)
	max   int
	start int // index of the oldest line within lines
	count int
}

func newScrollback(max int) *scrollback {
	return &scrollback{max: max}
}

// push copies row y of buf (columns [0,cols)) into the scrollback.
func (s *scrollback) push(buf *cell.Buffer, y, cols int) {
	if s.max <= 0 {
		return
	}
	if s.lines == nil {
		s.lines = make([][]cell.Cell, s.max)
	}
	row := make([]cell.Cell, cols)
	for x := range cols {
		row[x] = buf.At(x, y)
	}
	if s.count < s.max {
		s.lines[(s.start+s.count)%s.max] = row
		s.count++
	} else {
		s.lines[s.start] = row
		s.start = (s.start + 1) % s.max
	}
}

// Len returns the number of scrollback lines currently stored.
func (s *scrollback) Len() int { return s.count }

// Line returns the i'th scrollback line (0 = oldest), or nil if out of
// range.
func (s *scrollback) Line(i int) []cell.Cell {
	if i < 0 || i >= s.count {
		return nil
	}
	return s.lines[(s.start+i)%s.max]
}
