package widget

import (
	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

// TextAreaOptions configures TextArea.
type TextAreaOptions struct {
	Theme       style.Theme
	Placeholder string

	// Value is the field's *initial* content, read once at mount — see
	// TextArea's doc comment.
	Value string

	// OnChange, if non-nil, is called with the field's current content
	// after every edit.
	OnChange func(value string) tui.Msg
	// OnSubmit, if non-nil, is called with the field's content when
	// Ctrl+Enter is pressed (plain Enter inserts a newline, unlike
	// TextInput's Enter).
	OnSubmit func(value string) tui.Msg
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

	base := w.opts.Theme.Text()
	for row := range innerH {
		lineIdx := w.scrollRow + row
		if lineIdx >= len(lines) {
			break
		}
		ln := lines[lineIdx]
		for col := range innerW {
			idx := ln.start + w.scrollCol + col
			r := ' '
			if idx < ln.end {
				r = w.buf[idx]
			}
			style := base
			if idx == w.cursor && w.focused {
				style = cell.Style{Fg: base.Fg, Bg: base.Bg, Attr: base.Attr | cell.AttrReverse}
			}
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
	switch {
	case ke.Key == input.KeyLeft:
		w.cursor = max(w.cursor-1, 0)
	case ke.Key == input.KeyRight:
		w.cursor = min(w.cursor+1, len(w.buf))
	case ke.Key == input.KeyUp:
		w.moveVertical(-1)
	case ke.Key == input.KeyDown:
		w.moveVertical(1)
	case ke.Key == input.KeyHome:
		start, _ := lineBounds(w.buf, w.cursor)
		w.cursor = start
	case ke.Key == input.KeyEnd:
		_, end := lineBounds(w.buf, w.cursor)
		w.cursor = end

	case ke.Key == input.KeyBackspace:
		return w.backspace()
	case ke.Key == input.KeyDelete:
		return w.deleteForward()

	case ke.Key == input.KeyEnter: // Ctrl+Enter (OnSubmit) is intercepted before handleKey runs
		w.insertRune('\n')
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
// length if it's shorter — the standard editor convention.
func (w *textAreaWidget) moveVertical(delta int) {
	lineStart, lineEnd := lineBounds(w.buf, w.cursor)
	col := w.cursor - lineStart

	var targetStart, targetEnd int
	switch {
	case delta < 0 && lineStart == 0:
		w.cursor = 0
		return
	case delta < 0:
		targetStart, targetEnd = lineBounds(w.buf, lineStart-1)
	case delta > 0 && lineEnd == len(w.buf):
		w.cursor = len(w.buf)
		return
	case delta > 0:
		targetStart, targetEnd = lineBounds(w.buf, lineEnd+1)
	default:
		return
	}

	w.cursor = min(targetStart+col, targetEnd)
}

func (w *textAreaWidget) Focusable() bool         { return true }
func (w *textAreaWidget) SetFocused(focused bool) { w.focused = focused }
