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
	for col := range innerW {
		idx := w.scrollOffset + col
		r := ' '
		if idx < len(w.buf) {
			r = w.buf[idx]
		}
		style := base
		if idx == w.cursor && w.focused {
			style = cell.Style{Fg: base.Fg, Bg: base.Bg, Attr: base.Attr | cell.AttrReverse}
		}
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
	switch {
	case ke.Key == input.KeyLeft:
		w.cursor = max(w.cursor-1, 0)
	case ke.Key == input.KeyRight:
		w.cursor = min(w.cursor+1, len(w.buf))
	case ke.Key == input.KeyHome:
		w.cursor = 0
	case ke.Key == input.KeyEnd:
		w.cursor = len(w.buf)

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

func (w *textInputWidget) Focusable() bool         { return true }
func (w *textInputWidget) SetFocused(focused bool) { w.focused = focused }
