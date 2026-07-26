package cell

import "github.com/sandgorgon/tui/internal/wcwidth"

// Rect is an axis-aligned rectangle in cell coordinates. It's cell
// package's own minimal type for this — structurally similar to, but
// independent of, layout.Rect: layout sits above cell in the
// architecture (see docs/DESIGN.md §3), so cell can't depend on it.
// Widgets convert between the two at the boundary, a trivial field
// copy.
type Rect struct {
	X, Y, W, H int
}

// intersect returns the overlapping region of a and b. If they don't
// overlap, the result has W and H of 0.
func intersect(a, b Rect) Rect {
	x0, y0 := max(a.X, b.X), max(a.Y, b.Y)
	x1, y1 := min(a.X+a.W, b.X+b.W), min(a.Y+a.H, b.Y+b.H)
	if x1 < x0 {
		x1 = x0
	}
	if y1 < y0 {
		y1 = y0
	}
	return Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// Painter draws into a clipped, coordinate-translated region of a
// Buffer. Coordinates passed to its methods are relative to the
// Painter's own clip rectangle, so a widget scoped to a sub-Painter via
// Clip draws in local coordinates without knowing its absolute
// position in the buffer — the same composition model as a graphics
// canvas/context.
//
// Painter does not stack combining marks onto a preceding base
// character; a zero-width rune (see internal/wcwidth) is drawn as its
// own single-width cell. Real combining-mark composition is a known,
// deliberately deferred simplification.
type Painter struct {
	buf  *Buffer
	clip Rect
}

// NewPainter returns a Painter for the whole of buf.
func NewPainter(buf *Buffer) *Painter {
	return &Painter{buf: buf, clip: Rect{X: 0, Y: 0, W: buf.Width, H: buf.Height}}
}

// Clip returns a new Painter scoped to r (in the current Painter's
// local coordinates), intersected with the current clip so a child can
// never draw outside its parent's region.
func (p *Painter) Clip(r Rect) *Painter {
	abs := Rect{X: p.clip.X + r.X, Y: p.clip.Y + r.Y, W: r.W, H: r.H}
	return &Painter{buf: p.buf, clip: intersect(abs, p.clip)}
}

// Size returns the Painter's drawable area.
func (p *Painter) Size() (w, h int) { return p.clip.W, p.clip.H }

// SetCell writes r at (x,y) (local coordinates) with the given style,
// doing nothing if (x,y) is outside the clip. A wide rune (width 2)
// also writes a continuation cell at (x+1,y); if that would fall
// outside the clip, the rune isn't drawn at all rather than being
// truncated into a corrupted half-glyph.
func (p *Painter) SetCell(x, y int, r rune, style Style) {
	if x < 0 || y < 0 || x >= p.clip.W || y >= p.clip.H {
		return
	}
	width := wcwidth.RuneWidth(r)
	if width <= 0 {
		width = 1
	}
	if width == 2 && x+1 >= p.clip.W {
		return
	}

	ax, ay := p.clip.X+x, p.clip.Y+y
	p.buf.Set(ax, ay, Cell{Rune: r, Style: style, Width: uint8(width)})
	if width == 2 {
		p.buf.Set(ax+1, ay, Cell{Style: style, Width: 0})
	}
}

// SetRawCell writes c verbatim at (x,y) (local coordinates) — like
// Buffer.Set, with no wide-rune width computation — doing nothing if
// (x,y) is outside the clip. It's for compositing an already-resolved
// region of another Buffer into place (continuation cells and all),
// e.g. widget.Viewport blitting a scrolled window of its content
// buffer, or a future Terminal widget blitting a vt.Screen's buffer
// (see examples/multiplexer/compositor.go, which does the same thing
// by hand against a raw Buffer since it predates Painter having this).
// Most callers drawing new content want SetCell instead.
func (p *Painter) SetRawCell(x, y int, c Cell) {
	if x < 0 || y < 0 || x >= p.clip.W || y >= p.clip.H {
		return
	}
	p.buf.Set(p.clip.X+x, p.clip.Y+y, c)
}

// Text draws s on a single line starting at (x,y), with no wrapping
// (see the future Paragraph widget for that), clipped to the Painter's
// bounds. It returns the number of columns advanced, which may be less
// than the number of runes in s if a wide rune didn't fit.
func (p *Painter) Text(x, y int, s string, style Style) int {
	col := 0
	for _, r := range s {
		w := wcwidth.RuneWidth(r)
		if w <= 0 {
			w = 1
		}
		p.SetCell(x+col, y, r, style)
		col += w
	}
	return col
}

// Fill sets every cell in the rectangle [x,y)-[x+w,y+h) (local
// coordinates) to r/style, clipped to the Painter's bounds. r is
// typically a single-width character (e.g. a space for backgrounds, or
// a border-drawing character); Fill doesn't do wide-rune continuation
// bookkeeping the way SetCell/Text do.
func (p *Painter) Fill(x, y, w, h int, r rune, style Style) {
	rect := intersect(Rect{X: x, Y: y, W: w, H: h}, Rect{X: 0, Y: 0, W: p.clip.W, H: p.clip.H})
	width := wcwidth.RuneWidth(r)
	if width <= 0 {
		width = 1
	}
	c := Cell{Rune: r, Style: style, Width: uint8(width)}
	p.buf.Fill(p.clip.X+rect.X, p.clip.Y+rect.Y, rect.W, rect.H, c)
}
