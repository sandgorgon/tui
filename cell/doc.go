// Package cell defines the terminal cell grid: Cell, Style, Buffer, and
// the clipped-rect Painter that widgets draw into. Wide-rune widths are
// resolved via internal/wcwidth.
//
// See docs/DESIGN.md §4 for the design.
package cell
