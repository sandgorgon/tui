package widget

import "github.com/sandgorgon/tui/cell"

// centeredOverlay returns a Painter clipped to a box of at most
// wantW x wantH, centered within p (whose Size() is the full frame) —
// wantW/wantH <= 0 or larger than the frame means "use the whole
// frame". Shared by the two tui.OverlayPainter widgets in this
// package, Modal and CommandPalette. The returned Rect is in p's own
// local coordinates (same convention as Painter.Clip); callers that
// need to report it as tui.OverlayBounds rely on PaintOverlay always
// being handed an unclipped, whole-buffer Painter by the App, making
// local and absolute coordinates the same thing.
func centeredOverlay(p *cell.Painter, wantW, wantH int) (*cell.Painter, cell.Rect) {
	width, height := p.Size()
	w, h := wantW, wantH
	if w <= 0 || w > width {
		w = width
	}
	if h <= 0 || h > height {
		h = height
	}
	x, y := (width-w)/2, (height-h)/2
	r := cell.Rect{X: x, Y: y, W: w, H: h}
	return p.Clip(r), r
}

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
