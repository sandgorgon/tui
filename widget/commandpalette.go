package widget

import (
	"sort"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

// Command is one entry in a CommandPalette's list.
type Command struct {
	Label string
	// Data is opaque to CommandPalette; it's returned via OnSelect so
	// the caller can identify which command was chosen without needing
	// to match Label back up against its own list.
	Data any
}

// CommandPaletteOptions configures CommandPalette.
type CommandPaletteOptions struct {
	Theme       style.Theme
	Placeholder string

	// Open is whether the palette is currently shown.
	Open bool

	// Width and Height size the centered box; <= 0, or larger than the
	// frame, means "use the whole frame" — same convention as
	// ModalOptions.
	Width, Height int

	// OnSelect, if non-nil, is called with the chosen Command when
	// Enter picks one.
	OnSelect func(cmd Command) tui.Msg
	// OnCancel, if non-nil, is called when Esc is pressed.
	OnCancel func() tui.Msg
}

// CommandPalette is a centered, fuzzy-searchable command list overlay
// — an fzf-style "type to filter, arrow keys to navigate, Enter to
// pick" widget, built on the same tui.FocusScope/OverlayPainter
// mechanism as Modal (see its doc comment for why that's needed and
// what its limitations are).
//
// Unlike Modal, it's fully self-contained: the query text, the
// filtered/scored/sorted result list (see fuzzyMatch), and which
// result is under the cursor are all retained inside CommandPalette
// itself — none of it is business state the application needs mid-
// search, the same "in-progress edit buffer" reasoning as TextInput
// (docs/DESIGN.md §3.1). commands is the caller-supplied, static
// candidate list; the application only learns the outcome via
// OnSelect (Enter) or OnCancel (Esc) — it never sees the query as the
// user types it. Unlike Modal, whose body an app can choose to keep
// alive across opens, CommandPalette always resets its query and
// cursor to empty each time it transitions from closed to open — a
// stale search from last time is never the right default for a
// picker like this.
func CommandPalette(commands []Command, opts CommandPaletteOptions) tui.Node {
	return tui.Component(nil, commandPaletteProps{commands: commands, opts: opts}, func() tui.Widget {
		return &commandPaletteWidget{}
	})
}

type commandPaletteProps struct {
	commands []Command
	opts     CommandPaletteOptions
}

type scoredCommand struct {
	cmd     Command
	score   int
	matches []int
}

type commandPaletteWidget struct {
	commandPaletteProps
	editBuffer // the query text; cursor (promoted) is the text cursor, not list position

	resultCursor int
	scrollOffset int
	filtered     []scoredCommand

	// lastRect/rectSet back OverlayBounds — see modalWidget's identical
	// fields for why App needs this.
	lastRect cell.Rect
	rectSet  bool
}

func (w *commandPaletteWidget) Reconcile(props any) bool {
	wasOpen := w.opts.Open
	w.commandPaletteProps = props.(commandPaletteProps)

	if !w.opts.Open {
		if wasOpen {
			w.resetForNextOpen()
		}
		return true
	}
	if !wasOpen {
		w.resetForNextOpen()
	}
	w.refilter()
	return true
}

func (w *commandPaletteWidget) resetForNextOpen() {
	w.buf, w.cursor = nil, 0
	w.undo, w.redo = nil, nil
	w.resultCursor, w.scrollOffset = 0, 0
	w.filtered = nil
}

func (w *commandPaletteWidget) refilter() {
	query := string(w.buf)
	filtered := make([]scoredCommand, 0, len(w.commands))
	for _, cmd := range w.commands {
		score, matches, ok := fuzzyMatch(query, cmd.Label)
		if !ok {
			continue
		}
		filtered = append(filtered, scoredCommand{cmd: cmd, score: score, matches: matches})
	}
	sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].score > filtered[j].score })
	w.filtered = filtered

	if w.resultCursor >= len(w.filtered) {
		w.resultCursor = max(len(w.filtered)-1, 0)
	}
}

func (w *commandPaletteWidget) PaintOverlay(p *cell.Painter) {
	if !w.opts.Open {
		w.rectSet = false
		return
	}
	dialog, rect := centeredOverlay(p, w.opts.Width, w.opts.Height)
	w.lastRect, w.rectSet = rect, true
	width, height := dialog.Size()
	if width < 2 || height < 2 {
		return
	}

	bg := w.opts.Theme.Text()
	dialog.Fill(0, 0, width, height, ' ', bg)
	drawBorder(dialog, width, height, w.opts.Theme.FocusStyle())

	inner := dialog.Clip(cell.Rect{X: 1, Y: 1, W: max(width-2, 0), H: max(height-2, 0)})
	innerW, innerH := inner.Size()
	if innerW <= 0 || innerH <= 0 {
		return
	}

	w.paintQueryLine(inner, innerW, bg)

	listHeight := innerH - 1
	if listHeight <= 0 {
		return
	}
	w.paintResults(inner.Clip(cell.Rect{X: 0, Y: 1, W: innerW, H: listHeight}), bg)
}

func (w *commandPaletteWidget) paintQueryLine(p *cell.Painter, width int, bg cell.Style) {
	if len(w.buf) == 0 {
		if w.opts.Placeholder != "" {
			p.Text(0, 0, w.opts.Placeholder, w.opts.Theme.MutedText())
		}
		p.SetCell(0, 0, ' ', cell.Style{Attr: cell.AttrReverse})
		return
	}
	for col := 0; col < width && col < len(w.buf); col++ {
		style := bg
		if col == w.cursor {
			style = cell.Style{Fg: bg.Fg, Bg: bg.Bg, Attr: bg.Attr | cell.AttrReverse}
		}
		p.SetCell(col, 0, w.buf[col], style)
	}
	if w.cursor == len(w.buf) && w.cursor < width {
		p.SetCell(w.cursor, 0, ' ', cell.Style{Attr: cell.AttrReverse})
	}
}

func (w *commandPaletteWidget) paintResults(p *cell.Painter, bg cell.Style) {
	_, height := p.Size()
	if height <= 0 || len(w.filtered) == 0 {
		return
	}
	w.scrollOffset = clampScroll(w.scrollOffset, w.resultCursor, len(w.filtered), height)

	selected := cell.Style{Fg: bg.Fg, Bg: bg.Bg, Attr: bg.Attr | cell.AttrReverse}
	for row := range height {
		idx := w.scrollOffset + row
		if idx >= len(w.filtered) {
			break
		}
		sc := w.filtered[idx]

		rowStyle := bg
		marker := "  "
		if idx == w.resultCursor {
			rowStyle = selected
			marker = "> "
		}
		matchSet := make(map[int]bool, len(sc.matches))
		for _, m := range sc.matches {
			matchSet[m] = true
		}

		col := p.Text(0, row, marker, rowStyle)
		for i, r := range []rune(sc.cmd.Label) {
			cellStyle := rowStyle
			if matchSet[i] {
				cellStyle = cell.Style{Fg: w.opts.Theme.Primary, Bg: rowStyle.Bg, Attr: rowStyle.Attr | cell.AttrBold}
			}
			p.SetCell(col+i, row, r, cellStyle)
		}
	}
}

func (w *commandPaletteWidget) HandleEvent(e input.Event) tui.Cmd {
	ke, ok := e.(input.KeyEvent)
	if !ok {
		return nil
	}

	switch {
	case ke.Key == input.KeyEsc:
		if w.opts.OnCancel == nil {
			return nil
		}
		if msg := w.opts.OnCancel(); msg != nil {
			return func() tui.Msg { return msg }
		}
		return nil

	case ke.Key == input.KeyEnter:
		if len(w.filtered) == 0 || w.opts.OnSelect == nil {
			return nil
		}
		cmd := w.filtered[w.resultCursor].cmd
		if msg := w.opts.OnSelect(cmd); msg != nil {
			return func() tui.Msg { return msg }
		}
		return nil

	case ke.Key == input.KeyUp:
		w.resultCursor = max(w.resultCursor-1, 0)
	case ke.Key == input.KeyDown:
		w.resultCursor = min(w.resultCursor+1, max(len(w.filtered)-1, 0))

	case ke.Key == input.KeyLeft:
		w.cursor = max(w.cursor-1, 0)
	case ke.Key == input.KeyRight:
		w.cursor = min(w.cursor+1, len(w.buf))
	case ke.Key == input.KeyHome:
		w.cursor = 0
	case ke.Key == input.KeyEnd:
		w.cursor = len(w.buf)

	case ke.Key == input.KeyBackspace:
		if w.backspace() {
			w.refilter()
		}
	case ke.Key == input.KeyDelete:
		if w.deleteForward() {
			w.refilter()
		}

	case ke.Key == input.KeyNone && ke.Rune != 0 && ke.Mod&(input.ModCtrl|input.ModAlt) == 0:
		w.insertRune(ke.Rune)
		w.refilter()
	}
	return nil
}

// OverlayBounds reports the palette's absolute Rect from its last
// PaintOverlay — see tui.OverlayBounds.
func (w *commandPaletteWidget) OverlayBounds() (cell.Rect, bool) { return w.lastRect, w.rectSet }

// HandleOutsideClick implements tui.OutsideClicker by reusing OnCancel
// — for a command picker, clicking outside means the same thing as
// pressing Esc.
func (w *commandPaletteWidget) HandleOutsideClick(input.MouseEvent) tui.Cmd {
	if w.opts.OnCancel == nil {
		return nil
	}
	if msg := w.opts.OnCancel(); msg != nil {
		return func() tui.Msg { return msg }
	}
	return nil
}

// Focusable is false: per tui.FocusScope's doc comment, CommandPalette
// exposes itself as the sole entry from Focusables below rather than
// participating in the ordinary document-order focus walk.
func (w *commandPaletteWidget) Focusable() bool     { return false }
func (w *commandPaletteWidget) SetFocused(bool)     {}
func (w *commandPaletteWidget) Paint(*cell.Painter) {}

func (w *commandPaletteWidget) Active() bool { return w.opts.Open }
func (w *commandPaletteWidget) Focusables() []tui.Widget {
	return []tui.Widget{w}
}
