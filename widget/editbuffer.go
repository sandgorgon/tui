package widget

// undoState is one snapshot in an editBuffer's undo/redo history.
type undoState struct {
	buf    []rune
	cursor int
}

// editBuffer is the rune-buffer, cursor, and undo/redo state shared by
// TextInput and TextArea's editing — the canonical example
// docs/DESIGN.md §3.1 gives for state that belongs in a retained
// widget, not the application's Model (see TextInput's doc comment).
// Undo/redo is snapshot-based (a full copy of buf per edit) rather
// than diff-based: simplest-correct, and fine at the buffer sizes a
// single field or a screenful of text actually reaches.
type editBuffer struct {
	buf    []rune
	cursor int

	undo, redo []undoState
}

// mount sets the buffer's initial content, cursor at the end. Callers
// call this once, from their Widget.Reconcile's "first call" branch —
// see TextInput/TextArea's own mounted bookkeeping.
func (b *editBuffer) mount(value string) {
	b.buf = []rune(value)
	b.cursor = len(b.buf)
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

func (b *editBuffer) applyUndo() bool {
	if len(b.undo) == 0 {
		return false
	}
	b.redo = append(b.redo, b.snapshot())
	last := b.undo[len(b.undo)-1]
	b.undo = b.undo[:len(b.undo)-1]
	b.buf, b.cursor = last.buf, last.cursor
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
	return true
}

// insertRune inserts r at the cursor and advances it, recording undo
// history first. Call sites that insert several runes as one logical
// edit (insertString, paste) should pushUndo once themselves and call
// insertRuneNoUndo per rune instead, so undo reverts the whole edit in
// one step, not one keystroke at a time.
func (b *editBuffer) insertRune(r rune) {
	b.pushUndo()
	b.insertRuneNoUndo(r)
}

func (b *editBuffer) insertRuneNoUndo(r rune) {
	b.buf = insertRune(b.buf, b.cursor, r)
	b.cursor++
}

// insertString inserts s at the cursor as a single undo step.
func (b *editBuffer) insertString(s string) {
	if s == "" {
		return
	}
	b.pushUndo()
	for _, r := range s {
		b.insertRuneNoUndo(r)
	}
}

func (b *editBuffer) backspace() bool {
	if b.cursor == 0 {
		return false
	}
	b.pushUndo()
	b.buf = deleteRune(b.buf, b.cursor-1)
	b.cursor--
	return true
}

func (b *editBuffer) deleteForward() bool {
	if b.cursor >= len(b.buf) {
		return false
	}
	b.pushUndo()
	b.buf = deleteRune(b.buf, b.cursor)
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
