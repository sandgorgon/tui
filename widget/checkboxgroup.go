package widget

import (
	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

// CheckboxGroup is a focusable vertical list of independently
// toggleable options, rendered "[x] label" / "[ ] label", with cursor
// marking which row keyboard input (Up/Down/Space/Enter, typically)
// currently targets. Like RadioGroup, it has no scrolling machinery —
// short, fully-visible lists only — and no ephemeral state of its
// own: checked and cursor are both caller-owned business state,
// passed in fresh every frame (docs/DESIGN.md §3.1). Unlike
// RadioGroup, moving the cursor doesn't by itself change anything —
// checked state only changes when onEvent's caller decides it should
// (typically on Space/Enter), which is why cursor and checked are
// separate props here.
func CheckboxGroup(options []string, checked []bool, cursor int, theme style.Theme, onEvent func(input.Event) tui.Msg) tui.Node {
	return tui.Component(nil, checkboxGroupProps{
		options: options, checked: checked, cursor: cursor, theme: theme, onEvent: onEvent,
	}, func() tui.Widget {
		return &checkboxGroupWidget{}
	})
}

type checkboxGroupProps struct {
	options []string
	checked []bool
	cursor  int
	theme   style.Theme
	onEvent func(input.Event) tui.Msg
}

type checkboxGroupWidget struct {
	checkboxGroupProps
	focused bool
}

func (w *checkboxGroupWidget) Reconcile(props any) bool {
	w.checkboxGroupProps = props.(checkboxGroupProps)
	return true
}

func (w *checkboxGroupWidget) Paint(p *cell.Painter) {
	_, height := p.Size()
	base := w.theme.Text()
	for row, opt := range w.options {
		if row >= height {
			break
		}
		box := "[ ] "
		if row < len(w.checked) && w.checked[row] {
			box = "[x] "
		}

		rowStyle := base
		if row == w.cursor {
			rowStyle = cell.Style{Fg: base.Fg, Bg: base.Bg, Attr: base.Attr | cell.AttrReverse}
			if w.focused {
				rowStyle.Attr |= cell.AttrBold
			}
		}
		p.Text(0, row, box+opt, rowStyle)
	}
}

func (w *checkboxGroupWidget) HandleEvent(e input.Event) tui.Cmd {
	if w.onEvent == nil {
		return nil
	}
	msg := w.onEvent(e)
	if msg == nil {
		return nil
	}
	return func() tui.Msg { return msg }
}

func (w *checkboxGroupWidget) Focusable() bool         { return true }
func (w *checkboxGroupWidget) SetFocused(focused bool) { w.focused = focused }
