package widget

import (
	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

// TextInputOptions configures TextInput.
type TextInputOptions struct {
	Theme style.Theme

	// Placeholder is shown, in a muted style, whenever the field is
	// empty.
	Placeholder string

	// Value is the field's *initial* content, read once at mount — see
	// TextInput's doc comment.
	Value string

	// OnChange, if non-nil, is called with the field's current content
	// after every edit.
	OnChange func(value string) tui.Msg
	// OnSubmit, if non-nil, is called with the field's content when
	// Enter is pressed.
	OnSubmit func(value string) tui.Msg
}

// TextInput is a single-line, focusable, bordered text field with
// undo/redo. Its edit buffer, cursor position, and undo history (see
// editBuffer) are the canonical example docs/DESIGN.md §3.1 gives for
// state that belongs in a retained widget, not the application's
// Model: TextInput reads Value once, at mount, as the field's
// *initial* content, then owns all further editing itself — it is not
// resynced from Value on later frames (an uncontrolled component, in
// React terms). The application learns what's in the field via
// OnChange (after every edit) and OnSubmit (on Enter), not by reading
// anything back out of the Node.
func TextInput(opts TextInputOptions) tui.Node {
	return tui.Component(nil, opts, func() tui.Widget {
		return &textInputWidget{}
	})
}

type textInputWidget struct {
	opts    TextInputOptions
	mounted bool
	editBuffer

	scrollOffset int
	focused      bool

	// dragging is true between a MouseLeft press and its matching
	// release — see handleMouse.
	dragging bool
}

func (w *textInputWidget) Reconcile(props any) bool {
	w.opts = props.(TextInputOptions)
	if !w.mounted {
		w.mounted = true
		w.mount(w.opts.Value)
	}
	return true
}

func (w *textInputWidget) Paint(p *cell.Painter) {
	width, height := p.Size()
	if width < 2 || height < 2 {
		return
	}

	border := w.opts.Theme.BorderStyle()
	if w.focused {
		border = w.opts.Theme.FocusStyle()
	}
	drawBorder(p, width, height, border)

	inner := p.Clip(cell.Rect{X: 1, Y: 1, W: width - 2, H: 1})
	innerW, _ := inner.Size()
	if innerW <= 0 {
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

	switch {
	case w.cursor < w.scrollOffset:
		w.scrollOffset = w.cursor
	case w.cursor >= w.scrollOffset+innerW:
		w.scrollOffset = w.cursor - innerW + 1
	}
	if w.scrollOffset < 0 {
		w.scrollOffset = 0
	}

	base := w.opts.Theme.Text()
	selStart, selEnd, hasSel := w.selectionRange()
	for col := range innerW {
		idx := w.scrollOffset + col
		r := ' '
		if idx < len(w.buf) {
			r = w.buf[idx]
		}
		inSel := hasSel && idx >= selStart && idx < selEnd
		style := highlightStyle(base, w.focused, inSel, idx == w.cursor)
		inner.SetCell(col, 0, r, style)
	}
}

func (w *textInputWidget) HandleEvent(e input.Event) tui.Cmd {
	if ke, ok := e.(input.KeyEvent); ok && ke.Key == input.KeyEnter {
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
		w.insertString(stripNewlines(ev.Text))
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

// handleKey applies ke and reports whether it changed the buffer's
// content (as opposed to just moving the cursor, which never needs an
// OnChange notification).
func (w *textInputWidget) handleKey(ke input.KeyEvent) bool {
	shift := ke.Mod&input.ModShift != 0
	switch {
	case ke.Key == input.KeyLeft:
		w.moveHorizontal(-1, shift)
	case ke.Key == input.KeyRight:
		w.moveHorizontal(1, shift)
	case ke.Key == input.KeyHome:
		w.moveTo(0, shift)
	case ke.Key == input.KeyEnd:
		w.moveTo(len(w.buf), shift)

	case ke.Key == input.KeyBackspace:
		return w.backspace()
	case ke.Key == input.KeyDelete:
		return w.deleteForward()

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

// handleMouse implements click-to-position-cursor and click/drag-to-
// select: a plain press moves the cursor there and drops any
// selection; a Shift+press extends the active selection (starting one
// if needed) to the clicked point instead; while the button stays
// down, each further Drag event (and the final MouseRelease) starts a
// selection the first time the mouse actually moves and extends it to
// the new point — so a click with no movement in between never
// creates a (zero-width, effectively invisible) selection.
func (w *textInputWidget) handleMouse(me input.MouseEvent) {
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

// setCursorFromMouse moves the cursor to the buffer offset under
// me's local (X,Y), doing nothing if Y isn't the single content row
// (local row 1 — row 0 is the border).
func (w *textInputWidget) setCursorFromMouse(me input.MouseEvent) {
	if me.Y != 1 {
		return
	}
	w.cursor = clampCursor(w.scrollOffset+(me.X-1), len(w.buf))
}

func (w *textInputWidget) Focusable() bool         { return true }
func (w *textInputWidget) SetFocused(focused bool) { w.focused = focused }
