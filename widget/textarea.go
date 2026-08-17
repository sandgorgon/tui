package widget

import (
	"sort"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
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

	// OnChange, if non-nil, is called with the field's current content
	// after every edit.
	OnChange func(value string) tui.Msg
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
}

// TextArea is a multi-line, focusable, bordered text field with
// undo/redo, sharing TextInput's uncontrolled-after-mount design and
// editBuffer machinery (see TextInput's doc comment) — Value is read
// once, at mount, as the field's initial content; the application
// learns what's in it via OnChange and OnSubmit (Ctrl+Enter). Long
// lines scroll horizontally rather than soft-wrapping, so cursor
// movement always corresponds to what's on screen one-for-one; there's
// no line virtualization for very long documents (compare List's
// visible-window-only rendering) since a multi-line *editor*, unlike a
// read-only List, needs cursor math to stay simple and exact.
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

	// dragging is true between a MouseLeft press and its matching
	// release — see handleMouse.
	dragging bool
}

func (w *textAreaWidget) Reconcile(props any) bool {
	w.opts = props.(TextAreaOptions)
	if !w.mounted {
		w.mounted = true
		w.mount(w.opts.Value)
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

func (w *textAreaWidget) Paint(p *cell.Painter) {
	width, height := p.Size()
	if width < 2 || height < 2 {
		return
	}

	border := w.opts.Theme.BorderStyle()
	if w.focused {
		border = w.opts.Theme.FocusStyle()
	}
	drawBorder(p, width, height, border)

	inner := p.Clip(cell.Rect{X: 1, Y: 1, W: width - 2, H: height - 2})
	innerW, innerH := inner.Size()
	if innerW <= 0 || innerH <= 0 {
		return
	}

	if len(w.buf) == 0 {
		if w.opts.Placeholder != "" {
			inner.Text(0, 0, w.opts.Placeholder, w.opts.Theme.MutedText())
		}
		if w.focused {
			inner.SetCell(0, 0, ' ', cell.Style{Attr: cell.AttrReverse})
		}
		return
	}

	lines := splitLines(w.buf)
	cursorLine, cursorCol := 0, 0
	for i, ln := range lines {
		if w.cursor >= ln.start && w.cursor <= ln.end {
			cursorLine, cursorCol = i, w.cursor-ln.start
			break
		}
	}

	w.scrollRow = clampScroll(w.scrollRow, cursorLine, len(lines), innerH)
	switch {
	case cursorCol < w.scrollCol:
		w.scrollCol = cursorCol
	case cursorCol >= w.scrollCol+innerW:
		w.scrollCol = cursorCol - innerW + 1
	}
	if w.scrollCol < 0 {
		w.scrollCol = 0
	}

	defaultStyle := w.opts.Theme.Text()
	spans := w.opts.Highlights
	selStart, selEnd, hasSel := w.selectionRange()
	for row := range innerH {
		lineIdx := w.scrollRow + row
		if lineIdx >= len(lines) {
			break
		}
		ln := lines[lineIdx]
		// A line entirely swept by the selection (selection starts at
		// or before it and continues past its end into a later line)
		// highlights its whole row width, padding included, so a
		// multi-line selection reads as one continuous highlighted
		// block rather than stopping dead at each line's actual text.
		// A line the selection only starts or ends *within* only
		// highlights its real characters — extending that highlight
		// into the padding too would require re-deriving "how far
		// into a later line does the selection reach" from this row's
		// unrelated trailing columns, which don't correspond to real
		// buffer positions the way idx does for idx < ln.end.
		lineFullySelected := hasSel && selStart <= ln.start && selEnd > ln.end

		// idx jumps around non-monotonically row-to-row (a short line
		// following a long one, with scrollCol fixed from the long
		// line, can start well before the previous row's last idx), so
		// the span cursor is re-seeked once per row via binary search
		// rather than carried forward across rows — but still sweeps
		// forward (not per-cell binary search) within the row, since
		// idx is monotonic there.
		rowStart := ln.start + w.scrollCol
		spanIdx := sort.Search(len(spans), func(i int) bool { return spans[i].End > rowStart })
		for col := range innerW {
			idx := ln.start + w.scrollCol + col
			r := ' '
			if idx < ln.end {
				r = w.buf[idx]
			}
			for spanIdx < len(spans) && idx >= spans[spanIdx].End {
				spanIdx++
			}
			base := defaultStyle
			if spanIdx < len(spans) && idx >= spans[spanIdx].Start && idx < spans[spanIdx].End {
				base = spans[spanIdx].Style
			}
			inSel := lineFullySelected || (hasSel && idx < ln.end && idx >= selStart && idx < selEnd)
			style := highlightStyle(base, w.focused, inSel, idx == w.cursor && idx <= ln.end)
			inner.SetCell(col, row, r, style)
		}
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

	changed := false
	switch ev := e.(type) {
	case input.KeyEvent:
		changed = w.handleKey(ev)
	case input.PasteEvent:
		w.insertString(ev.Text) // newlines allowed, unlike TextInput's paste
		changed = true
	case input.MouseEvent:
		w.handleMouse(ev)
		return nil // cursor/selection changes aren't content edits, no OnChange
	default:
		return nil
	}

	if !changed || w.opts.OnChange == nil {
		return nil
	}
	msg := w.opts.OnChange(string(w.buf))
	if msg == nil {
		return nil
	}
	return func() tui.Msg { return msg }
}

func (w *textAreaWidget) handleKey(ke input.KeyEvent) bool {
	shift := ke.Mod&input.ModShift != 0
	switch {
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

// moveVertical moves the cursor to the equivalent column on the
// previous (delta<0) or next (delta>0) line, clamped to that line's
// length if it's shorter — the standard editor convention. shift
// extends (or starts) the active selection to the new position, same
// as every other movement here — see moveTo.
func (w *textAreaWidget) moveVertical(delta int, shift bool) {
	lineStart, lineEnd := lineBounds(w.buf, w.cursor)
	col := w.cursor - lineStart

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

	w.moveTo(min(targetStart+col, targetEnd), shift)
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
	lines := splitLines(w.buf)
	lineIdx := max(0, min(w.scrollRow+(me.Y-1), len(lines)-1))
	ln := lines[lineIdx]

	idx := ln.start + w.scrollCol + (me.X - 1)
	w.cursor = max(ln.start, min(idx, ln.end))
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
