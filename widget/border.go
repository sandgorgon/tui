package widget

import "github.com/sandgorgon/tui/cell"

// drawBorder paints a single-line box border in style around the
// [0,0)-[width,height) rectangle of p, shared by every widget in this
// package that draws its own focus border (List, Viewport) instead of
// requiring a tui.Focusable wrapper.
func drawBorder(p *cell.Painter, width, height int, style cell.Style) {
	if width < 2 || height < 2 {
		return
	}
	p.SetCell(0, 0, '┌', style)
	p.SetCell(width-1, 0, '┐', style)
	p.SetCell(0, height-1, '└', style)
	p.SetCell(width-1, height-1, '┘', style)
	for x := 1; x < width-1; x++ {
		p.SetCell(x, 0, '─', style)
		p.SetCell(x, height-1, '─', style)
	}
	for y := 1; y < height-1; y++ {
		p.SetCell(0, y, '│', style)
		p.SetCell(width-1, y, '│', style)
	}
}
