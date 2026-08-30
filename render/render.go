package render

import (
	"io"
	"strconv"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/term"
)

// defaultMergeThreshold is the default gap (in cells) within a changed
// row below which two separate change-spans are merged into one,
// re-emitting the unchanged cells between them instead of paying for a
// second cursor-repositioning escape sequence (roughly 6-9 bytes; see
// docs/DESIGN.md §3.2). Options.MergeThreshold overrides this.
const defaultMergeThreshold = 8

// Options configures a Renderer for a specific terminal's capabilities
// and link characteristics.
type Options struct {
	// ColorLevel controls color downsampling: truecolor cells get
	// reduced to 256 or 16 colors if the terminal can't display more.
	ColorLevel term.ColorLevel

	// SynchronizedOutput wraps each frame in DEC mode 2026 begin/end,
	// eliminating visible tearing on terminals that support it (see
	// term.Capabilities.SynchronizedOutput, probed via term.Probe).
	SynchronizedOutput bool

	// MergeThreshold overrides defaultMergeThreshold if positive. A
	// larger value trades more re-emitted bytes for fewer cursor moves
	// — worth raising over a high-latency link (see docs/DESIGN.md
	// §3.2), though nothing in this package measures latency itself;
	// a caller that wants to adapt it dynamically may.
	MergeThreshold int
}

func (o Options) mergeThreshold() int {
	if o.MergeThreshold > 0 {
		return o.MergeThreshold
	}
	return defaultMergeThreshold
}

// Renderer diffs successive cell.Buffer frames and writes the minimal
// ANSI/SGR byte sequence needed to bring a terminal from one frame to
// the next. It keeps its own copy of what it believes the terminal
// currently displays — not an alias of the caller's buffer, since the
// caller typically mutates and re-passes the same buffer object frame
// to frame (see docs/DESIGN.md §3.2 and §10: this is also the
// encoding half of the render<->vt round-trip correctness harness).
//
// A Renderer is not safe for concurrent use.
type Renderer struct {
	opts Options

	front *cell.Buffer // what we believe the terminal currently displays

	cursorX, cursorY int
	cursorVisible    bool
	curStyle         cell.Style

	buf []byte // scratch, reused across Render calls
}

// NewRenderer returns a ready-to-use Renderer. The first call to
// Render always does a full repaint, since there's no prior frame to
// diff against.
func NewRenderer(opts Options) *Renderer {
	return &Renderer{opts: opts, cursorX: -1, cursorY: -1, cursorVisible: true}
}

// Render computes the diff between the Renderer's remembered previous
// frame and back, writes the minimal byte sequence to w that updates
// the terminal accordingly, positions the cursor at (cursorX,cursorY)
// with the given visibility, and remembers back's content as the new
// front for the next call. It writes nothing at all if there's no
// difference to report.
func (r *Renderer) Render(w io.Writer, back *cell.Buffer, cursorX, cursorY int, cursorVisible bool) error {
	resized := r.front == nil || r.front.Width != back.Width || r.front.Height != back.Height
	if resized {
		r.front = cell.NewBuffer(back.Width, back.Height)
		r.cursorX, r.cursorY = -1, -1
	}

	r.buf = r.buf[:0]
	if r.opts.SynchronizedOutput {
		r.buf = append(r.buf, "\x1b[?2026h"...)
	}

	if resized {
		// front was just reset to blank on the assumption that's what
		// the real terminal shows here too. That assumption only holds
		// on the very first frame (into a freshly entered alt screen);
		// on a genuine size change, a real terminal does NOT clear its
		// existing content on its own (confirmed on VTE/gnome-terminal),
		// so without this, any cell blank in both the reset front and
		// the new back frame is skipped as "already correct" and stale
		// pixels from a differently-sized previous frame keep showing
		// through. Erase explicitly rather than relying on front's
		// reset value to stand in for the terminal's real content.
		//
		// Erase in Display paints its blanks using the terminal's
		// current graphic rendition (real terminals do this so programs
		// can paint colored blank areas), not some assumed default, so
		// reset that rendition first — otherwise a cell last written in
		// a bright color would erase to a bright blank instead of a
		// plain one.
		if !styleEqualSGR(cell.Style{}, r.curStyle) {
			r.buf = appendSGRDiff(r.buf, r.curStyle, cell.Style{}, r.opts.ColorLevel)
		}
		if r.curStyle.Hyperlink != "" {
			r.buf = appendHyperlink(r.buf, "")
		}
		r.curStyle = cell.Style{}
		r.buf = append(r.buf, "\x1b[2J"...)
	}

	threshold := r.opts.mergeThreshold()
	for y := range back.Height {
		r.renderRow(y, back, threshold)
	}

	r.placeCursor(cursorX, cursorY, cursorVisible)

	if r.opts.SynchronizedOutput {
		r.buf = append(r.buf, "\x1b[?2026l"...)
	}

	if len(r.buf) == 0 {
		return nil
	}
	_, err := w.Write(r.buf)
	return err
}

func (r *Renderer) placeCursor(x, y int, visible bool) {
	if visible {
		if r.cursorX != x || r.cursorY != y {
			r.buf = appendCursorMove(r.buf, x, y)
			r.cursorX, r.cursorY = x, y
		}
		if !r.cursorVisible {
			r.buf = append(r.buf, "\x1b[?25h"...)
			r.cursorVisible = true
		}
	} else if r.cursorVisible {
		r.buf = append(r.buf, "\x1b[?25l"...)
		r.cursorVisible = false
	}
}

func (r *Renderer) renderRow(y int, back *cell.Buffer, threshold int) {
	// Cheap skip: a direct cell-by-cell comparison, bailing on the
	// first difference. This is what docs/DESIGN.md §3.2 means by
	// skipping unchanged rows cheaply — an actual rolling hash would
	// still cost O(cols) to compute per row (every cell must be read
	// at least once either way) without saving the memory a full diff
	// needs anyway for rows that *do* change, so it wouldn't earn its
	// complexity here.
	changed := false
	for x := range back.Width {
		if r.front.At(x, y) != back.At(x, y) {
			changed = true
			break
		}
	}
	if !changed {
		return
	}

	for _, sp := range computeSpans(r.front, back, y, threshold) {
		r.emitSpan(sp.start, sp.end, y, back)
	}
}

type span struct{ start, end int }

// computeSpans finds the column ranges within row y that differ
// between front and back, merging spans separated by a gap of at most
// threshold cells (see defaultMergeThreshold).
func computeSpans(front, back *cell.Buffer, y, threshold int) []span {
	var diffCols []int
	for x := range back.Width {
		if front.At(x, y) != back.At(x, y) {
			diffCols = append(diffCols, x)
		}
	}
	if len(diffCols) == 0 {
		return nil
	}

	spans := make([]span, 0, len(diffCols))
	start, end := diffCols[0], diffCols[0]+1
	for _, x := range diffCols[1:] {
		if x-end <= threshold {
			end = x + 1
			continue
		}
		spans = append(spans, span{start, end})
		start, end = x, x+1
	}
	return append(spans, span{start, end})
}

// emitSpan writes cells [x0,x1) of row y, repositioning the cursor
// first only if it isn't already known to be at x0,y, and updates
// Renderer's bookkeeping (front, cursor position, current style) to
// match. It's width-aware: a wide rune's continuation cell is skipped
// as output (the terminal advances past both columns on its own) but
// still copied into front.
func (r *Renderer) emitSpan(x0, x1, y int, back *cell.Buffer) {
	if r.cursorX != x0 || r.cursorY != y {
		r.buf = appendCursorMove(r.buf, x0, y)
	}

	x := x0
	for x < x1 {
		c := back.At(x, y)
		if c.Width == 0 { // a continuation cell reached directly: shouldn't normally happen, skip defensively
			r.front.Set(x, y, c)
			x++
			continue
		}

		if !styleEqualSGR(c.Style, r.curStyle) {
			r.buf = appendSGRDiff(r.buf, r.curStyle, c.Style, r.opts.ColorLevel)
		}
		if c.Style.Hyperlink != r.curStyle.Hyperlink {
			r.buf = appendHyperlink(r.buf, c.Style.Hyperlink)
		}
		r.curStyle = c.Style

		rn := c.Rune
		if rn == 0 {
			rn = ' '
		}
		r.buf = append(r.buf, string(rn)...)

		r.front.Set(x, y, c)
		w := int(c.Width)
		if w == 2 && x+1 < back.Width {
			r.front.Set(x+1, y, back.At(x+1, y))
		}
		if w < 1 {
			w = 1
		}
		x += w
	}

	r.cursorX, r.cursorY = x, y
}

func appendCursorMove(buf []byte, x, y int) []byte {
	buf = append(buf, "\x1b["...)
	buf = strconv.AppendInt(buf, int64(y+1), 10)
	buf = append(buf, ';')
	buf = strconv.AppendInt(buf, int64(x+1), 10)
	return append(buf, 'H')
}
