package widget

import "github.com/sandgorgon/tui/cell"

// undoState is one snapshot in an editBuffer's undo/redo history.
type undoState struct {
	buf    []rune
	cursor int
}

// editBuffer is the rune-buffer, cursor, undo/redo, and selection
// state shared by TextInput and TextArea's editing — the canonical
// example docs/DESIGN.md §3.1 gives for state that belongs in a
// retained widget, not the application's Model (see TextInput's doc
// comment). Undo/redo is snapshot-based (a full copy of buf per edit)
// rather than diff-based: simplest-correct, and fine at the buffer
// sizes a single field or a screenful of text actually reaches.
type editBuffer struct {
	buf    []rune
	cursor int

	// hasSelection/selAnchor track an in-progress or completed text
	// selection: selAnchor is the fixed end, cursor is the moving end.
	// A collapsed selection (selAnchor == cursor) doesn't count as
	// active for rendering/replace-on-type purposes — see
	// selectionRange — matching every mainstream editor's convention
	// that a zero-width selection is the same as no selection.
	hasSelection bool
	selAnchor    int

	undo, redo []undoState
}

// mount sets the buffer's initial content, cursor at the end. Callers
// call this once, from their Widget.Reconcile's "first call" branch —
// see TextInput/TextArea's own mounted bookkeeping.
func (b *editBuffer) mount(value string) {
	b.buf = []rune(value)
	b.cursor = len(b.buf)
	b.hasSelection = false
}

// selectionRange returns the ordered [start,end) rune-offset range of
// the current selection, and whether one is actually active (i.e.
// non-empty — see hasSelection's doc comment).
func (b *editBuffer) selectionRange() (start, end int, ok bool) {
	if !b.hasSelection {
		return 0, 0, false
	}
	start, end = b.selAnchor, b.cursor
	if start > end {
		start, end = end, start
	}
	return start, end, start < end
}

// startSelection anchors a new selection at the cursor's current
// position, if one isn't already active — called before a Shift+
// movement key, or the start of a mouse drag, changes the cursor.
func (b *editBuffer) startSelection() {
	if !b.hasSelection {
		b.selAnchor = b.cursor
		b.hasSelection = true
	}
}

func (b *editBuffer) clearSelection() {
	b.hasSelection = false
}

// removeSelectionNoUndo deletes the current selection's runes without
// recording its own undo step, for insertRune/insertString to fold
// into the single undo step they push for the insertion that follows.
// Reports whether there was a selection to remove.
func (b *editBuffer) removeSelectionNoUndo() bool {
	start, end, ok := b.selectionRange()
	if !ok {
		return false
	}
	b.buf = append(b.buf[:start], b.buf[end:]...)
	b.cursor = start
	b.hasSelection = false
	return true
}

// deleteSelection removes the active selection as its own undo step —
// for Backspace/Delete when a selection is active, which delete the
// whole selection instead of one character. Reports whether it did
// anything (so backspace/deleteForward can fall back to their normal
// single-character behavior when there's no selection).
func (b *editBuffer) deleteSelection() bool {
	if _, _, ok := b.selectionRange(); !ok {
		return false
	}
	b.pushUndo()
	b.removeSelectionNoUndo()
	return true
}

// moveTo sets the cursor to pos: with shift, extends (or starts) the
// active selection to include the movement; without it, drops any
// selection first — Home/End/Up/Down's "just move, drop the
// selection" case (deliberately simpler than moveHorizontal's
// collapse-to-edge convention: a widget library, not a full editor).
func (b *editBuffer) moveTo(pos int, shift bool) {
	if shift {
		b.startSelection()
	} else {
		b.clearSelection()
	}
	b.cursor = pos
}

// moveHorizontal moves the cursor one rune in delta's direction
// (delta<0 left, delta>0 right), clamped to [0,len(buf)]. Without
// shift, if a selection is active it collapses the cursor to whichever
// edge is in delta's direction instead of moving one rune from the
// cursor's current position — the standard editor convention for
// Left/Right with an active selection (browsers and most editors do
// this; Up/Down/Home/End don't get the same treatment here, see
// moveTo).
func (b *editBuffer) moveHorizontal(delta int, shift bool) {
	if shift {
		b.startSelection()
		b.cursor = clampCursor(b.cursor+delta, len(b.buf))
		return
	}
	if start, end, ok := b.selectionRange(); ok {
		b.clearSelection()
		if delta < 0 {
			b.cursor = start
		} else {
			b.cursor = end
		}
		return
	}
	b.cursor = clampCursor(b.cursor+delta, len(b.buf))
}

func clampCursor(pos, max int) int {
	if pos < 0 {
		return 0
	}
	if pos > max {
		return max
	}
	return pos
}

func (b *editBuffer) snapshot() undoState {
	return undoState{buf: append([]rune(nil), b.buf...), cursor: b.cursor}
}

// pushUndo records the buffer's current state before a destructive
// edit, and invalidates the redo stack (a new edit forks history away
// from whatever was undone).
func (b *editBuffer) pushUndo() {
	b.undo = append(b.undo, b.snapshot())
	b.redo = nil
}

// applyUndo/applyRedo drop any active selection along with restoring
// buf/cursor — selAnchor isn't part of undoState, and re-deriving
// "what was selected" after jumping to an unrelated point in history
// isn't meaningful anyway; every mainstream editor drops the selection
// on undo/redo too.
func (b *editBuffer) applyUndo() bool {
	if len(b.undo) == 0 {
		return false
	}
	b.redo = append(b.redo, b.snapshot())
	last := b.undo[len(b.undo)-1]
	b.undo = b.undo[:len(b.undo)-1]
	b.buf, b.cursor = last.buf, last.cursor
	b.hasSelection = false
	return true
}

func (b *editBuffer) applyRedo() bool {
	if len(b.redo) == 0 {
		return false
	}
	b.undo = append(b.undo, b.snapshot())
	last := b.redo[len(b.redo)-1]
	b.redo = b.redo[:len(b.redo)-1]
	b.buf, b.cursor = last.buf, last.cursor
	b.hasSelection = false
	return true
}

// insertRune inserts r at the cursor and advances it, recording undo
// history first — replacing the active selection (if any) as part of
// the same undo step, the standard "typing over a selection replaces
// it" convention. Call sites that insert several runes as one logical
// edit (insertString, paste) should pushUndo once themselves and call
// insertRuneNoUndo per rune instead, so undo reverts the whole edit in
// one step, not one keystroke at a time.
func (b *editBuffer) insertRune(r rune) {
	b.pushUndo()
	b.removeSelectionNoUndo()
	b.insertRuneNoUndo(r)
}

func (b *editBuffer) insertRuneNoUndo(r rune) {
	b.buf = insertRune(b.buf, b.cursor, r)
	b.cursor++
}

// insertString inserts s at the cursor as a single undo step,
// replacing the active selection (if any) first — see insertRune.
func (b *editBuffer) insertString(s string) {
	if s == "" {
		return
	}
	b.pushUndo()
	b.removeSelectionNoUndo()
	for _, r := range s {
		b.insertRuneNoUndo(r)
	}
}

// backspace deletes the active selection if there is one (see
// deleteSelection), otherwise the single rune before the cursor.
func (b *editBuffer) backspace() bool {
	if b.deleteSelection() {
		return true
	}
	if b.cursor == 0 {
		return false
	}
	b.pushUndo()
	b.buf = deleteRune(b.buf, b.cursor-1)
	b.cursor--
	return true
}

// deleteForward deletes the active selection if there is one (see
// deleteSelection), otherwise the single rune after the cursor.
func (b *editBuffer) deleteForward() bool {
	if b.deleteSelection() {
		return true
	}
	if b.cursor >= len(b.buf) {
		return false
	}
	b.pushUndo()
	b.buf = deleteRune(b.buf, b.cursor)
	return true
}

// deleteWordBackward deletes the active selection if there is one (see
// deleteSelection), otherwise from the cursor back to the previous
// word boundary (see wordBoundary) — Ctrl+Backspace's conventional
// meaning in most editors.
func (b *editBuffer) deleteWordBackward() bool {
	if b.deleteSelection() {
		return true
	}
	if b.cursor == 0 {
		return false
	}
	b.pushUndo()
	start := wordBoundary(b.buf, b.cursor, -1)
	b.buf = append(b.buf[:start], b.buf[b.cursor:]...)
	b.cursor = start
	return true
}

// deleteWordForward deletes the active selection if there is one (see
// deleteSelection), otherwise from the cursor forward to the next word
// boundary (see wordBoundary) — Ctrl+Delete's conventional meaning.
func (b *editBuffer) deleteWordForward() bool {
	if b.deleteSelection() {
		return true
	}
	if b.cursor >= len(b.buf) {
		return false
	}
	b.pushUndo()
	end := wordBoundary(b.buf, b.cursor, 1)
	b.buf = append(b.buf[:b.cursor], b.buf[end:]...)
	return true
}

func insertRune(buf []rune, at int, r rune) []rune {
	tail := make([]rune, len(buf)-at)
	copy(tail, buf[at:])
	out := append(buf[:at], r)
	return append(out, tail...)
}

func deleteRune(buf []rune, at int) []rune {
	return append(buf[:at], buf[at+1:]...)
}

func stripNewlines(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r == '\n' || r == '\r' {
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

// highlightStyle returns a reverse-video style — the same convention
// this codebase already uses for a selected List/Table row — if this
// cell is part of the active selection or is the raw cursor position,
// while focused; otherwise base unchanged. Shared by TextInput and
// TextArea's Paint. A selection deliberately doesn't get its own
// distinct look from the plain single-cursor case (both just reverse
// video): once a selection is active, the whole range renders
// uniformly highlighted with no separate caret glyph inside it, the
// same convention vim's visual mode uses.
func highlightStyle(base cell.Style, focused, inSelection, isCursor bool) cell.Style {
	if !focused || !(inSelection || isCursor) {
		return base
	}
	return cell.Style{Fg: base.Fg, Bg: base.Bg, Attr: base.Attr | cell.AttrReverse}
}
