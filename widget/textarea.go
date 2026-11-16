package widget

import (
	"sort"
	"unicode"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/internal/wcwidth"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

// StyleSpan overrides the style of buffer positions [Start, End) —
// e.g. a syntax-highlighted token, a diagnostic range, or a search
// match. Spans must be sorted by Start and must not overlap; Paint
// sweeps them once per visible row assuming this, so out-of-order or
// overlapping spans produce undefined (not crashing) results. A span
// composes with the existing selection/cursor highlight rather than
// being replaced by it, the same way the caller's own theme colors
// already do.
type StyleSpan struct {
	Start, End int
	Style      cell.Style
}

// TextAreaOptions configures TextArea.
type TextAreaOptions struct {
	Theme       style.Theme
	Placeholder string

	// Value is the field's *initial* content, read once at mount — see
	// TextArea's doc comment.
	Value string

	// Highlights overrides the base style of specific buffer ranges —
	// see StyleSpan. Recomputed and passed fresh every frame, like any
	// other prop; nil means no overrides.
	Highlights []StyleSpan

	// InitialCursor, if non-nil, places the cursor at this rune offset
	// into Value when the field mounts, instead of the end of Value
	// (the nil/default behavior) — e.g. jumping to a specific line/
	// column when opening an existing document. Read once at mount,
	// like Value; the pointed-to value is clamped to
	// [0, len([]rune(Value))]. A pointer (not a plain int) because 0 is
	// itself a meaningful offset (start of the field) distinct from
	// "unset" — a plain int couldn't tell "place the cursor at the
	// start" apart from "caller didn't set this."
	InitialCursor *int

	// Gutter, if non-nil, paints a fixed-width column to the left of
	// each visible line's text — e.g. a line number ("42") or a
	// single-glyph diagnostic marker. Called once per visible row each
	// frame (not once per document line), so a caller deriving it from
	// something dynamic (search results, diff state) stays cheap even
	// on a large document. The column's width is the widest string
	// returned across the currently visible rows, plus one separator
	// column, so it can change frame to frame as the document scrolls;
	// strings are right-aligned within it, space-padded on the left.
	// A row whose Gutter string doesn't fit the visible height (there
	// is none — Gutter is only ever asked about in-range lineIdx
	// values) never occurs.
	Gutter func(lineIdx int) (string, cell.Style)

	// OnChange, if non-nil, is called with the field's current content
	// after every edit.
	OnChange func(value string) tui.Msg
	// OnCursorChange, if non-nil, is called with the cursor's current
	// rune offset — the same unit InitialCursor takes — whenever it
	// moves: a navigation key, a mouse click/drag, or an edit that also
	// relocates it. Unlike OnChange, this fires on pure cursor movement
	// too (arrow keys, Home/End, a click) since that isn't a content
	// edit. Lets a caller track the position across a remount (e.g.
	// restoring it via InitialCursor after the field is later
	// recreated) or drive a live "line N, col M" indicator.
	OnCursorChange func(offset int) tui.Msg
	// OnSelectionChange, if non-nil, is called with the active
	// selection's ordered [start, end) rune-offset range and whether
	// one is actually active — the same shape
	// editBuffer.selectionRange() returns internally — whenever it
	// changes: extending/shrinking via Shift+movement or a mouse drag,
	// or being dropped by a plain movement or edit. ok is false (with
	// start and end both 0) when there is no active selection. Lets a
	// caller implement e.g. a clipboard-copy command or a "N
	// characters selected" indicator without maintaining its own copy
	// of the anchor/cursor state.
	OnSelectionChange func(start, end int, ok bool) tui.Msg
	// OnSubmit, if non-nil, is called with the field's content when
	// Ctrl+Enter is pressed (plain Enter inserts a newline, unlike
	// TextInput's Enter).
	OnSubmit func(value string) tui.Msg

	// ReleaseKey is the key that exits the field and resumes Tab/
	// Shift-Tab focus navigation — TextArea always claims raw Tab (see
	// tui.RawKeyClaimer) so a literal tab character can be typed, so it
	// needs its own way out. The zero value defaults to Esc, which
	// TextArea has no independent use for.
	ReleaseKey input.KeyEvent

	// Frameless, when true, skips drawing the border and uses the full
	// painted rect as content area instead of insetting by one cell —
	// for a caller that already visually separates TextArea from its
	// surroundings (e.g. its own Box margin) and would otherwise get a
	// second, redundant layer of chrome. Cursor rendering is unaffected
	// since it doesn't depend on the border. Defaults to false,
	// preserving the bordered behavior every existing caller already
	// gets.
	Frameless bool

	// SoftWrap, when true, word-wraps each line that's wider than the
	// text area across as many screen rows as it needs, instead of
	// scrolling it horizontally (the default). Display only: the
	// buffer, OnChange's value, and every offset (InitialCursor,
	// OnCursorChange, Highlights) are unchanged. Lines break after a
	// space or tab; a word wider than the whole row is broken mid-word.
	// The rightmost column is kept free for the end-of-line cursor (and
	// for one trailing space that doesn't fit, which "hangs" there
	// rather than starting the next row). Up/Down and PageUp/PageDown
	// move by screen row, keeping the cursor's column; Home/End still
	// go to the start/end of the whole line, not the screen row. Gutter
	// is asked about each line once, for the line's first row; its
	// continuation rows get a blank gutter in that line's style. Can be
	// toggled between frames — the scroll position re-clamps to keep
	// the cursor visible.
	SoftWrap bool
}

// TextArea is a multi-line, focusable, bordered (optionally frameless
// — see TextAreaOptions.Frameless) text field with undo/redo, sharing
// TextInput's uncontrolled-after-mount design and
// editBuffer machinery (see TextInput's doc comment) — Value is read
// once, at mount, as the field's initial content; the application
// learns what's in it via OnChange and OnSubmit (Ctrl+Enter). By
// default long lines scroll horizontally, so each screen row is
// exactly one buffer line; TextAreaOptions.SoftWrap word-wraps them
// instead. There's no line virtualization for very long documents
// (compare List's visible-window-only rendering) since a multi-line
// *editor*, unlike a read-only List, needs cursor math to stay simple
// and exact.
func TextArea(opts TextAreaOptions) tui.Node {
	return tui.Component(nil, opts, func() tui.Widget {
		return &textAreaWidget{}
	})
}

type textAreaWidget struct {
	opts    TextAreaOptions
	mounted bool
	editBuffer

	scrollRow, scrollCol int
	focused              bool

	// lastVisibleLines is innerH from the most recent Paint — the only
	// way PageUp/PageDown's handleKey cases know how many lines to jump
	// by, since Paint's own innerH is computed fresh each frame from the
	// painter's Size() and otherwise never retained. Zero before the
	// first Paint; handleKey falls back to moving by 1 line in that case.
	lastVisibleLines int

	// lastTextW/lastGutterW are the text column count and gutter width
	// from the most recent Paint, for the same reason as
	// lastVisibleLines: key and mouse handling run outside Paint but
	// need them — soft-wrapped rows depend on the text width, and a
	// click's column has to skip the gutter. Zero before the first
	// Paint; soft-wrapped movement then falls back to moving by buffer
	// line.
	lastTextW, lastGutterW int

	// dragging is true between a MouseLeft press and its matching
	// release — see handleMouse.
	dragging bool
}

func (w *textAreaWidget) Reconcile(props any) bool {
	w.opts = props.(TextAreaOptions)
	if !w.mounted {
		w.mounted = true
		w.mount(w.opts.Value)
		if w.opts.InitialCursor != nil {
			w.cursor = clampCursor(*w.opts.InitialCursor, len(w.buf))
		}
	}
	return true
}

// textLine is one line's [start,end) rune range within an editBuffer's
// flat buf — TextArea stores content as a single rune slice with '\n'
// as the line separator (reusing editBuffer's insert/delete/undo as-is
// rather than a [][]rune structure), and derives lines from it here.
type textLine struct{ start, end int }

func splitLines(buf []rune) []textLine {
	lines := make([]textLine, 0, 1)
	start := 0
	for i, r := range buf {
		if r == '\n' {
			lines = append(lines, textLine{start, i})
			start = i + 1
		}
	}
	return append(lines, textLine{start, len(buf)})
}

// lineBounds returns the [start,end) range of the line containing
// index i.
func lineBounds(buf []rune, i int) (start, end int) {
	start, end = i, i
	for start > 0 && buf[start-1] != '\n' {
		start--
	}
	for end < len(buf) && buf[end] != '\n' {
		end++
	}
	return start, end
}

// runeCols returns r's on-screen column width, matching what
// cell.Painter.SetCell will actually store for it (a control rune that
// SetCell substitutes a placeholder for still occupies one column, not
// the negative width wcwidth.RuneWidth reports for the raw rune).
func runeCols(r rune) int {
	w := wcwidth.RuneWidth(r)
	if w <= 0 {
		return 1
	}
	return w
}

// visualWidth returns the number of on-screen columns buf[from:to]
// occupies — the same quantity Paint's per-line column math has to
// agree with, so a wide rune (e.g. CJK) counts as 2 rune-buffer
// columns' worth of screen space, not 1.
func visualWidth(buf []rune, from, to int) int {
	w := 0
	for _, r := range buf[from:to] {
		w += runeCols(r)
	}
	return w
}

// columnToIndex returns the buffer index within [from,to) whose glyph
// occupies visual column targetCol relative to from, or to if targetCol
// falls at or past the line's content — the inverse of visualWidth,
// used to turn a screen column (a mouse click, or a vertical-move's
// preserved column) back into a buffer position.
func columnToIndex(buf []rune, from, to, targetCol int) int {
	col := 0
	idx := from
	for idx < to {
		w := runeCols(buf[idx])
		if col+w > targetCol {
			return idx
		}
		col += w
		idx++
	}
	return to
}

// lineIndexOf returns the index into lines of the line containing
// buffer offset pos — shared by Paint (to find the cursor's row) and
// movePage (to jump by a line count rather than lineBounds' one-line-
// at-a-time walk).
func lineIndexOf(lines []textLine, pos int) int {
	for i, ln := range lines {
		if pos >= ln.start && pos <= ln.end {
			return i
		}
	}
	return len(lines) - 1
}

// visualRow is one screen row of a soft-wrapped TextArea: the [start,
// end) slice of buffer line `line` it shows, and whether it's that
// line's final row (the only row that can show the cursor at the
// line's end — on any other row, its end offset is the next row's
// start and the cursor there is drawn on the next row).
type visualRow struct {
	seg  textLine
	line int
	last bool
}

// isWrapSpace reports whether a soft wrap may break right after r.
func isWrapSpace(r rune) bool { return r == ' ' || r == '\t' }

// wrapLine splits ln into the segments soft wrap shows on successive
// rows of width columns. Content fills at most width-1 columns, leaving
// the last one for the end-of-line cursor or one hanging trailing
// space (see TextAreaOptions.SoftWrap) — except at width 1, where
// there's no column to spare. A segment ends after the last space or
// tab that fits; a run with no such break (one long word) is broken
// where it overflows, always taking at least one rune so wrapping
// progresses even when a single wide rune is wider than the row. If
// the final segment still fills the whole row (only possible at those
// degenerate widths), an empty segment follows it so the end-of-line
// cursor has somewhere to go.
func wrapLine(buf []rune, ln textLine, width int) []textLine {
	limit := width - 1
	if limit < 1 {
		limit = width
	}
	var segs []textLine
	start, col, brk := ln.start, 0, -1
	for i := ln.start; i < ln.end; {
		r := buf[i]
		rw := runeCols(r)
		switch {
		case col+rw <= limit:
			col += rw
			i++
			if isWrapSpace(r) {
				brk = i
			}
		case isWrapSpace(r) && col+rw <= width:
			i++ // hang it in the reserved last column, then break
			segs = append(segs, textLine{start, i})
			start, col, brk = i, 0, -1
		default:
			end := i
			switch {
			case brk > start:
				end = brk
			case i == start:
				end = i + 1
			}
			segs = append(segs, textLine{start, end})
			start, col, brk = end, 0, -1
			i = end
		}
	}
	segs = append(segs, textLine{start, ln.end})
	if start < ln.end && visualWidth(buf, start, ln.end) >= width {
		segs = append(segs, textLine{ln.end, ln.end})
	}
	return segs
}

// wrapRows soft-wraps every line of buf to width columns.
func wrapRows(buf []rune, lines []textLine, width int) []visualRow {
	rows := make([]visualRow, 0, len(lines))
	for li, ln := range lines {
		segs := wrapLine(buf, ln, width)
		for si, seg := range segs {
			rows = append(rows, visualRow{seg: seg, line: li, last: si == len(segs)-1})
		}
	}
	return rows
}

// rowIndexOf returns the index of the row showing buffer offset pos:
// the last row starting at or before it, so an offset on a wrap
// boundary belongs to the row it starts, not the one it ends.
func rowIndexOf(rows []visualRow, pos int) int {
	i := sort.Search(len(rows), func(i int) bool { return rows[i].seg.start > pos })
	return max(i-1, 0)
}

// rowColumnToIndex is columnToIndex for one soft-wrapped row: past the
// row's content it clamps to the row's last rune rather than its end
// offset, unless this is the line's last row — that end offset is the
// next row's start, so landing on it would put the cursor on the wrong
// row.
func rowColumnToIndex(buf []rune, row visualRow, col int) int {
	idx := columnToIndex(buf, row.seg.start, row.seg.end, col)
	if !row.last && idx == row.seg.end && idx > row.seg.start {
		idx--
	}
	return idx
}

// softWrapRows returns the soft-wrapped rows for key/mouse handling,
// at the most recent Paint's text width — or ok=false when soft wrap
// is off or nothing has been painted yet, meaning: move by buffer
// line.
func (w *textAreaWidget) softWrapRows() (rows []visualRow, ok bool) {
	if !w.opts.SoftWrap || w.lastTextW <= 0 {
		return nil, false
	}
	return wrapRows(w.buf, splitLines(w.buf), w.lastTextW), true
}

// gutterTextWidth returns s's on-screen column width, matching
// visualWidth's wide-rune handling.
func gutterTextWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeCols(r)
	}
	return w
}

// gutterWidth returns the fixed column width this frame's visible
// rows need for gutter — the widest string any of them returns, plus
// one separator column — or 0 if gutter is nil or every visible row's
// string is empty. Recomputed each frame from only the rows actually
// on screen (lineIdx in [scrollRow, scrollRow+innerH)), not the whole
// document, so a caller deriving Gutter from something dynamic stays
// cheap even on a large document.
func gutterWidth(gutter func(int) (string, cell.Style), lineCount, scrollRow, innerH int) int {
	if gutter == nil {
		return 0
	}
	maxW := 0
	for row := range innerH {
		lineIdx := scrollRow + row
		if lineIdx >= lineCount {
			break
		}
		text, _ := gutter(lineIdx)
		if w := gutterTextWidth(text); w > maxW {
			maxW = w
		}
	}
	if maxW == 0 {
		return 0
	}
	return maxW + 1
}

// paintGutterRow right-aligns gutter's string for lineIdx within
// columns [0,gutterW) of row row — gutterW-1 columns of (space-padded)
// text plus one trailing blank separator column, matching
// gutterWidth's "widest string plus one" accounting. Padding and the
// separator column paint in the row's own returned style (blank
// glyph) so a caller painting the gutter as one colored panel via
// its Bg doesn't get a seam of the default background running
// through the padding/separator cells.
func paintGutterRow(p *cell.Painter, gutter func(int) (string, cell.Style), lineIdx, row, gutterW int) {
	text, style := gutter(lineIdx)
	paintGutterCells(p, text, style, row, gutterW)
}

// paintGutterCells is paintGutterRow's painting half, taking the text
// and style directly — soft wrap paints a continuation row's gutter as
// blank text in its line's style.
func paintGutterCells(p *cell.Painter, text string, style cell.Style, row, gutterW int) {
	textW := gutterTextWidth(text)
	col := 0
	for ; col < gutterW-1-textW; col++ {
		p.SetCell(col, row, ' ', style)
	}
	for _, r := range text {
		p.SetCell(col, row, r, style)
		col += runeCols(r)
	}
	for ; col < gutterW; col++ {
		p.SetCell(col, row, ' ', style)
	}
}

func (w *textAreaWidget) Paint(p *cell.Painter) {
	width, height := p.Size()
	minSize := 2 // room for drawBorder's corners
	if w.opts.Frameless {
		minSize = 1
	}
	if width < minSize || height < minSize {
		return
	}

	inner := p
	if !w.opts.Frameless {
		border := w.opts.Theme.BorderStyle()
		if w.focused {
			border = w.opts.Theme.FocusStyle()
		}
		drawBorder(p, width, height, border)
		inner = p.Clip(cell.Rect{X: 1, Y: 1, W: width - 2, H: height - 2})
	}
	innerW, innerH := inner.Size()
	if innerW <= 0 || innerH <= 0 {
		return
	}
	w.lastVisibleLines = innerH

	if len(w.buf) == 0 {
		gutterW := gutterWidth(w.opts.Gutter, 1, 0, 1)
		if gutterW > 0 && gutterW < innerW {
			paintGutterRow(inner, w.opts.Gutter, 0, 0, gutterW)
		} else {
			gutterW = 0
		}
		w.lastTextW, w.lastGutterW = innerW-gutterW, gutterW
		if w.opts.Placeholder != "" {
			inner.Text(gutterW, 0, w.opts.Placeholder, w.opts.Theme.MutedText())
		}
		if w.focused {
			inner.SetCell(gutterW, 0, ' ', cell.Style{Attr: cell.AttrReverse})
		}
		return
	}

	lines := splitLines(w.buf)
	if w.opts.SoftWrap {
		w.paintWrapped(inner, innerW, innerH, lines)
		return
	}
	cursorLine := lineIndexOf(lines, w.cursor)
	cursorCol := visualWidth(w.buf, lines[cursorLine].start, w.cursor)

	w.scrollRow = clampScroll(w.scrollRow, cursorLine, len(lines), innerH)

	gutterW := gutterWidth(w.opts.Gutter, len(lines), w.scrollRow, innerH)
	textW := innerW - gutterW
	if textW < 1 {
		gutterW, textW = 0, innerW
	}
	w.lastTextW, w.lastGutterW = textW, gutterW

	switch {
	case cursorCol < w.scrollCol:
		w.scrollCol = cursorCol
	case cursorCol >= w.scrollCol+textW:
		w.scrollCol = cursorCol - textW + 1
	}
	if w.scrollCol < 0 {
		w.scrollCol = 0
	}

	for row := range innerH {
		lineIdx := w.scrollRow + row
		if lineIdx >= len(lines) {
			break
		}
		if gutterW > 0 {
			paintGutterRow(inner, w.opts.Gutter, lineIdx, row, gutterW)
		}
		w.paintSegment(inner, row, gutterW, textW, lines[lineIdx], w.scrollCol, true)
	}
}

// paintWrapped is Paint for TextAreaOptions.SoftWrap: the same per-row
// painting as the horizontally scrolling layout, but over soft-wrapped
// rows (see wrapRows) with no horizontal scroll, and scrollRow counting
// screen rows rather than buffer lines.
//
// The text width the rows wrap to depends on the gutter's width, which
// unwrapped Paint sizes from the lines on screen — but which lines are
// on screen depends on the wrap. To break that cycle, the gutter is
// sized from every line that *could* be on screen: the cursor's row is
// always visible and every line takes at least one row, so no visible
// line is more than innerH lines from the cursor's. That can only make
// the gutter wider than strictly needed (e.g. one digit early, just
// before line 100 scrolls into view), never too narrow.
func (w *textAreaWidget) paintWrapped(inner *cell.Painter, innerW, innerH int, lines []textLine) {
	cursorLine := lineIndexOf(lines, w.cursor)
	from := max(0, cursorLine-innerH)
	gutterW := gutterWidth(w.opts.Gutter, len(lines), from, cursorLine-from+innerH+1)
	textW := innerW - gutterW
	if textW < 1 {
		gutterW, textW = 0, innerW
	}
	w.lastTextW, w.lastGutterW = textW, gutterW
	w.scrollCol = 0

	rows := wrapRows(w.buf, lines, textW)
	w.scrollRow = clampScroll(w.scrollRow, rowIndexOf(rows, w.cursor), len(rows), innerH)

	for row := range innerH {
		ri := w.scrollRow + row
		if ri >= len(rows) {
			break
		}
		vr := rows[ri]
		if gutterW > 0 {
			text, style := w.opts.Gutter(vr.line)
			if ri > 0 && rows[ri-1].line == vr.line {
				text = "" // continuation row: blank, in the line's own style
			}
			paintGutterCells(inner, text, style, row, gutterW)
		}
		w.paintSegment(inner, row, gutterW, textW, vr.seg, 0, vr.last)
	}
}

// paintSegment paints buffer range seg onto screen row row, in the
// textW columns right of the gutter, skipping its first skipCols
// columns (horizontal scroll). cursorAtEnd says whether the cursor may
// be drawn at seg.end — true for a whole line or a line's last wrapped
// row, false for an earlier wrapped row, whose end offset is drawn as
// the next row's start.
func (w *textAreaWidget) paintSegment(inner *cell.Painter, row, gutterW, textW int, seg textLine, skipCols int, cursorAtEnd bool) {
	defaultStyle := w.opts.Theme.Text()
	spans := w.opts.Highlights
	selStart, selEnd, hasSel := w.selectionRange()
	// A segment entirely swept by the selection (selection starts
	// at or before it and continues past its end into a later row)
	// highlights its whole row width, padding included, so a
	// multi-line selection reads as one continuous highlighted
	// block rather than stopping dead at each row's actual text.
	// A segment the selection only starts or ends *within* only
	// highlights its real characters — extending that highlight
	// into the padding too would require re-deriving "how far
	// into a later row does the selection reach" from this row's
	// unrelated trailing columns, which don't correspond to real
	// buffer positions the way idx does for idx < seg.end.
	lineFullySelected := hasSel && selStart <= seg.start && selEnd > seg.end

	// idx starts at the first buffer rune at or past the scrolled-
	// past region rather than jumping there by arithmetic (seg.start
	// + skipCols, valid only when every rune is one column wide):
	// a wide rune earlier in the line shifts a buffer index's screen
	// column away from its rune-count offset, so finding where
	// skipCols lands means walking runes and summing width. If
	// skipCols splits a wide rune (lands mid-glyph), that rune is
	// skipped entirely — col then overshoots skipCols by the
	// glyph's leftover column(s), which the loop below renders
	// blank rather than drawing half a glyph.
	idx, col := seg.start, 0
	for idx < seg.end && col < skipCols {
		col += runeCols(w.buf[idx])
		idx++
	}

	// The span cursor is re-seeked once per row via binary search
	// rather than carried forward across rows, since idx jumps
	// around non-monotonically row-to-row (a short line following a
	// long one, with scrollCol fixed from the long line, can start
	// well before the previous row's last idx) — but still sweeps
	// forward (not per-cell binary search) within the row, since idx
	// is monotonic there.
	spanIdx := sort.Search(len(spans), func(i int) bool { return spans[i].End > idx })
	for screenCol := 0; screenCol < textW; {
		if gap := col - skipCols; gap > screenCol {
			style := highlightStyle(defaultStyle, w.focused, lineFullySelected, false)
			inner.SetCell(gutterW+screenCol, row, ' ', style)
			screenCol++
			continue
		}

		atEnd := idx >= seg.end
		r := rune(' ')
		width := 1
		if !atEnd {
			r = w.buf[idx]
			width = runeCols(r)
		}
		if screenCol+width > textW {
			// A wide rune that would only half-fit at the row's
			// right edge: cell.Painter.SetCell won't draw it at all
			// (see its own doc comment), but this column still needs
			// *some* explicit content this frame, or a stale cell
			// from a previous frame survives undrawn.
			style := highlightStyle(defaultStyle, w.focused, lineFullySelected, false)
			inner.SetCell(gutterW+screenCol, row, ' ', style)
			screenCol++
			continue
		}
		for spanIdx < len(spans) && idx >= spans[spanIdx].End {
			spanIdx++
		}
		base := defaultStyle
		if !atEnd && spanIdx < len(spans) && idx >= spans[spanIdx].Start && idx < spans[spanIdx].End {
			base = spans[spanIdx].Style
		}
		inSel := lineFullySelected || (hasSel && !atEnd && idx >= selStart && idx < selEnd)
		isCursor := idx == w.cursor && (idx < seg.end || (idx == seg.end && cursorAtEnd))
		style := highlightStyle(base, w.focused, inSel, isCursor)
		inner.SetCell(gutterW+screenCol, row, r, style)

		screenCol += width
		col += width
		idx++
	}
}

func (w *textAreaWidget) HandleEvent(e input.Event) tui.Cmd {
	if ke, ok := e.(input.KeyEvent); ok && ke.Key == input.KeyEnter && ke.Mod&input.ModCtrl != 0 {
		if w.opts.OnSubmit == nil {
			return nil
		}
		msg := w.opts.OnSubmit(string(w.buf))
		if msg == nil {
			return nil
		}
		return func() tui.Msg { return msg }
	}

	prevCursor := w.cursor
	prevSelStart, prevSelEnd, prevHasSel := w.selectionRange()
	changed := false
	switch ev := e.(type) {
	case input.KeyEvent:
		changed = w.handleKey(ev)
	case input.PasteEvent:
		w.insertString(ev.Text) // newlines allowed, unlike TextInput's paste
		changed = true
	case input.MouseEvent:
		w.handleMouse(ev) // cursor/selection changes aren't content edits, no OnChange
	default:
		return nil
	}

	var cmds []tui.Cmd
	if changed && w.opts.OnChange != nil {
		if msg := w.opts.OnChange(string(w.buf)); msg != nil {
			cmds = append(cmds, func() tui.Msg { return msg })
		}
	}
	if w.cursor != prevCursor && w.opts.OnCursorChange != nil {
		if msg := w.opts.OnCursorChange(w.cursor); msg != nil {
			cmds = append(cmds, func() tui.Msg { return msg })
		}
	}
	if w.opts.OnSelectionChange != nil {
		if selStart, selEnd, hasSel := w.selectionRange(); selStart != prevSelStart || selEnd != prevSelEnd || hasSel != prevHasSel {
			if msg := w.opts.OnSelectionChange(selStart, selEnd, hasSel); msg != nil {
				cmds = append(cmds, func() tui.Msg { return msg })
			}
		}
	}
	return tui.Batch(cmds...)
}

func (w *textAreaWidget) handleKey(ke input.KeyEvent) bool {
	shift := ke.Mod&input.ModShift != 0
	ctrl := ke.Mod&input.ModCtrl != 0
	switch {
	case ctrl && ke.Key == input.KeyLeft:
		w.moveTo(wordBoundary(w.buf, w.cursor, -1), shift)
	case ctrl && ke.Key == input.KeyRight:
		w.moveTo(wordBoundary(w.buf, w.cursor, 1), shift)
	case ctrl && ke.Key == input.KeyHome:
		w.moveTo(0, shift)
	case ctrl && ke.Key == input.KeyEnd:
		w.moveTo(len(w.buf), shift)
	case ctrl && (ke.Key == input.KeyUp || ke.Key == input.KeyDown || ke.Key == input.KeyPgUp || ke.Key == input.KeyPgDown):
		// Deliberately a no-op, not a fallthrough to plain vertical/
		// page movement: this library assigns no meaning to Ctrl+Up/
		// Down/PgUp/PgDown, unlike Ctrl+Left/Right/Home/End's word/
		// buffer jumps above, so these combos are genuinely left
		// unclaimed rather than silently treated as a plain move. A
		// host app is free to bind them to its own behavior (e.g.
		// switching which TextArea is focused, or tab-switching —
		// Ctrl+PgUp/PgDown's conventional meaning in browsers/editors)
		// without TextArea also reacting to the same keystroke
		// underneath it.

	case ke.Key == input.KeyLeft:
		w.moveHorizontal(-1, shift)
	case ke.Key == input.KeyRight:
		w.moveHorizontal(1, shift)
	case ke.Key == input.KeyUp:
		w.moveVertical(-1, shift)
	case ke.Key == input.KeyDown:
		w.moveVertical(1, shift)
	case ke.Key == input.KeyHome:
		start, _ := lineBounds(w.buf, w.cursor)
		w.moveTo(start, shift)
	case ke.Key == input.KeyEnd:
		_, end := lineBounds(w.buf, w.cursor)
		w.moveTo(end, shift)
	case ke.Key == input.KeyPgUp:
		w.movePage(-1, shift)
	case ke.Key == input.KeyPgDown:
		w.movePage(1, shift)

	case ctrl && ke.Key == input.KeyBackspace:
		return w.deleteWordBackward()
	case ctrl && ke.Key == input.KeyDelete:
		return w.deleteWordForward()
	case ke.Key == input.KeyBackspace:
		return w.backspace()
	case ke.Key == input.KeyDelete:
		return w.deleteForward()

	case ke.Key == input.KeyEnter: // Ctrl+Enter (OnSubmit) is intercepted before handleKey runs
		w.insertRune('\n')
		return true

	case ke.Key == input.KeyTab: // reaches here at all only because WantsRawTab claims it — see ReleaseKey
		w.insertRune('\t')
		return true

	case ke.Mod&input.ModCtrl != 0 && ke.Rune == 'z':
		return w.applyUndo()
	case ke.Mod&input.ModCtrl != 0 && ke.Rune == 'y':
		return w.applyRedo()

	case ke.Key == input.KeyNone && ke.Rune != 0 && ke.Mod&(input.ModCtrl|input.ModAlt) == 0:
		w.insertRune(ke.Rune)
		return true
	}
	return false
}

// moveVertical moves the cursor to the equivalent screen column on the
// previous (delta<0) or next (delta>0) line, clamped to that line's
// length if it's shorter — the standard editor convention, preserving
// visual column (not buffer-rune count) so the cursor stays under the
// same on-screen position even when a wide rune (e.g. CJK) on one of
// the two lines shifts rune-count and screen column out of step. shift
// extends (or starts) the active selection to the new position, same
// as every other movement here — see moveTo. Under SoftWrap it moves
// by screen row instead (see moveRows).
func (w *textAreaWidget) moveVertical(delta int, shift bool) {
	if rows, ok := w.softWrapRows(); ok {
		w.moveRows(rows, delta, false, shift)
		return
	}
	lineStart, lineEnd := lineBounds(w.buf, w.cursor)
	col := visualWidth(w.buf, lineStart, w.cursor)

	var targetStart, targetEnd int
	switch {
	case delta < 0 && lineStart == 0:
		w.moveTo(0, shift)
		return
	case delta < 0:
		targetStart, targetEnd = lineBounds(w.buf, lineStart-1)
	case delta > 0 && lineEnd == len(w.buf):
		w.moveTo(len(w.buf), shift)
		return
	case delta > 0:
		targetStart, targetEnd = lineBounds(w.buf, lineEnd+1)
	default:
		return
	}

	w.moveTo(columnToIndex(w.buf, targetStart, targetEnd, col), shift)
}

// movePage moves the cursor by one screenful of lines (dir<0 up,
// dir>0 down), same visual-column-clamping convention as moveVertical
// — it's moveVertical's shape with a multi-line delta instead of ±1.
// Falls back to a single line if lastVisibleLines hasn't been set yet
// (i.e. before the first Paint). Under SoftWrap a screenful is
// counted in screen rows (see moveRows).
func (w *textAreaWidget) movePage(dir int, shift bool) {
	n := w.lastVisibleLines
	if n <= 0 {
		n = 1
	}
	if rows, ok := w.softWrapRows(); ok {
		w.moveRows(rows, dir*n, true, shift)
		return
	}

	lines := splitLines(w.buf)
	lineStart, _ := lineBounds(w.buf, w.cursor)
	col := visualWidth(w.buf, lineStart, w.cursor)

	target := lineIndexOf(lines, w.cursor) + dir*n
	target = max(0, min(target, len(lines)-1))

	ln := lines[target]
	w.moveTo(columnToIndex(w.buf, ln.start, ln.end, col), shift)
}

// moveRows is moveVertical/movePage for soft wrap: it moves the
// cursor delta screen rows, keeping its column within the row (see
// rowColumnToIndex for where a too-short row clamps it). Moving past
// the first/last row goes to the start/end of the buffer, like
// moveVertical on the first/last line — or, with clamp (paging), stops
// on the first/last row like movePage.
func (w *textAreaWidget) moveRows(rows []visualRow, delta int, clamp, shift bool) {
	ri := rowIndexOf(rows, w.cursor)
	col := visualWidth(w.buf, rows[ri].seg.start, w.cursor)
	target := ri + delta
	switch {
	case clamp:
		target = max(0, min(target, len(rows)-1))
	case target < 0:
		w.moveTo(0, shift)
		return
	case target >= len(rows):
		w.moveTo(len(w.buf), shift)
		return
	}
	w.moveTo(rowColumnToIndex(w.buf, rows[target], col), shift)
}

// isWordChar reports whether r is part of a "word" for Ctrl+Left/
// Ctrl+Right's word-boundary jump — letters, digits, and underscore.
func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// wordBoundary scans from from in delta's direction (delta<0 left,
// delta>0 right) past the leading run of non-word characters, then
// past the following run of word characters, and returns where it
// stopped — the standard "skip whitespace/punctuation, then skip the
// word" editor convention. '\n' counts as a non-word character like
// any other, so a jump can cross a line boundary the same as it would
// cross a run of spaces.
func wordBoundary(buf []rune, from int, delta int) int {
	i := from
	if delta > 0 {
		for i < len(buf) && !isWordChar(buf[i]) {
			i++
		}
		for i < len(buf) && isWordChar(buf[i]) {
			i++
		}
		return i
	}
	for i > 0 && !isWordChar(buf[i-1]) {
		i--
	}
	for i > 0 && isWordChar(buf[i-1]) {
		i--
	}
	return i
}

// handleMouse implements click-to-position-cursor and click/drag-to-
// select — see textInputWidget.handleMouse's identical doc comment for
// the click-vs-drag-vs-Shift+click convention; setCursorFromMouse is
// TextArea's own multi-line version of computing a buffer offset from
// (X,Y).
func (w *textAreaWidget) handleMouse(me input.MouseEvent) {
	switch {
	case me.Button == input.MouseLeft && !me.Drag:
		w.dragging = true
		if me.Mod&input.ModShift == 0 {
			w.clearSelection()
		} else {
			w.startSelection()
		}
		w.setCursorFromMouse(me)
	case w.dragging && me.Button == input.MouseLeft && me.Drag:
		w.startSelection()
		w.setCursorFromMouse(me)
	case w.dragging && me.Button == input.MouseRelease:
		w.startSelection()
		w.setCursorFromMouse(me)
		w.dragging = false
	}
}

// setCursorFromMouse translates me's local (X,Y) into a buffer offset
// and moves the cursor there, clamping to the nearest valid line/
// column if the point falls outside the currently-painted content
// (e.g. past the last line — or, since App keeps delivering mouse
// events to whichever widget holds focus even once a drag leaves that
// widget's own on-screen bounds (see tui.App.hitTest's doc comment), a
// raw, untranslated absolute coordinate arriving mid-drag). Unlike
// Table's column-resize drag (see its handleColumnDrag), this is safe
// to just clamp rather than needing to detect and abandon the drag
// outright: each event computes an absolute buffer position fresh from
// (X,Y) rather than accumulating a delta from the previous event, so
// one stray untranslated event produces at most one wrong-but-harmless
// cursor placement for that single frame, not a compounding error.
func (w *textAreaWidget) setCursorFromMouse(me input.MouseEvent) {
	x, y := me.X, me.Y
	if !w.opts.Frameless {
		x--
		y-- // top/left border
	}

	x -= w.lastGutterW // a click on the gutter itself lands on column 0

	if rows, ok := w.softWrapRows(); ok {
		ri := max(0, min(w.scrollRow+y, len(rows)-1))
		w.cursor = rowColumnToIndex(w.buf, rows[ri], max(0, x))
		return
	}

	lines := splitLines(w.buf)
	lineIdx := max(0, min(w.scrollRow+y, len(lines)-1))
	ln := lines[lineIdx]

	targetCol := max(0, w.scrollCol+x)
	w.cursor = columnToIndex(w.buf, ln.start, ln.end, targetCol)
}

func (w *textAreaWidget) Focusable() bool         { return true }
func (w *textAreaWidget) SetFocused(focused bool) { w.focused = focused }

// WantsRawTab and ReleaseKey implement tui.RawKeyClaimer — see
// TextAreaOptions.ReleaseKey.
func (w *textAreaWidget) WantsRawTab() bool { return true }
func (w *textAreaWidget) ReleaseKey() input.KeyEvent {
	if w.opts.ReleaseKey != (input.KeyEvent{}) {
		return w.opts.ReleaseKey
	}
	return input.KeyEvent{Key: input.KeyEsc}
}
