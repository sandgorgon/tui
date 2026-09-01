package widget

import (
	"strings"
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

func textInputApp(t *testing.T, opts TextInputOptions) *tui.App {
	t.Helper()
	m := &widgetHostModel{node: TextInput(opts)}
	return tui.NewApp(m, 12, 3)
}

func TestTextInputTypingInsertsAtCursor(t *testing.T) {
	app := textInputApp(t, TextInputOptions{Theme: style.DefaultDark()})
	for _, r := range "hi" {
		app.HandleInput(input.KeyEvent{Rune: r})
	}
	rows := strings.Split(app.Buffer().String(), "\n")
	if !strings.Contains(rows[1], "hi") {
		t.Errorf("row = %q, want to contain \"hi\"", rows[1])
	}
}

func TestTextInputBackspaceAndDelete(t *testing.T) {
	var value string
	app := textInputApp(t, TextInputOptions{
		Theme: style.DefaultDark(),
		OnChange: func(v string) tui.Msg {
			value = v
			return nil
		},
	})
	for _, r := range "cat" {
		app.HandleInput(input.KeyEvent{Rune: r})
	}
	app.HandleInput(input.KeyEvent{Key: input.KeyBackspace})
	if value != "ca" {
		t.Fatalf("value after backspace = %q, want %q", value, "ca")
	}

	app.HandleInput(input.KeyEvent{Key: input.KeyLeft})
	app.HandleInput(input.KeyEvent{Key: input.KeyLeft})
	app.HandleInput(input.KeyEvent{Key: input.KeyDelete})
	if value != "a" {
		t.Fatalf("value after delete at start = %q, want %q", value, "a")
	}
}

func TestTextInputUndoRedo(t *testing.T) {
	var value string
	app := textInputApp(t, TextInputOptions{
		Theme: style.DefaultDark(),
		OnChange: func(v string) tui.Msg {
			value = v
			return nil
		},
	})
	for _, r := range "abc" {
		app.HandleInput(input.KeyEvent{Rune: r})
	}
	if value != "abc" {
		t.Fatalf("value = %q, want %q", value, "abc")
	}

	app.HandleInput(input.KeyEvent{Rune: 'z', Mod: input.ModCtrl})
	if value != "ab" {
		t.Fatalf("value after undo = %q, want %q", value, "ab")
	}
	app.HandleInput(input.KeyEvent{Rune: 'z', Mod: input.ModCtrl})
	if value != "a" {
		t.Fatalf("value after second undo = %q, want %q", value, "a")
	}
	app.HandleInput(input.KeyEvent{Rune: 'y', Mod: input.ModCtrl})
	if value != "ab" {
		t.Fatalf("value after redo = %q, want %q", value, "ab")
	}
}

func TestTextInputNewEditClearsRedoStack(t *testing.T) {
	var value string
	app := textInputApp(t, TextInputOptions{
		Theme: style.DefaultDark(),
		OnChange: func(v string) tui.Msg {
			value = v
			return nil
		},
	})
	for _, r := range "ab" {
		app.HandleInput(input.KeyEvent{Rune: r})
	}
	app.HandleInput(input.KeyEvent{Rune: 'z', Mod: input.ModCtrl}) // undo -> "a"
	app.HandleInput(input.KeyEvent{Rune: 'x'})                     // new edit -> "ax", should clear redo
	if value != "ax" {
		t.Fatalf("value = %q, want %q", value, "ax")
	}
	app.HandleInput(input.KeyEvent{Rune: 'y', Mod: input.ModCtrl}) // redo should be a no-op now
	if value != "ax" {
		t.Fatalf("value after redo-with-empty-stack = %q, want unchanged %q", value, "ax")
	}
}

func TestTextInputOnSubmit(t *testing.T) {
	var submitted string
	app := textInputApp(t, TextInputOptions{
		Theme: style.DefaultDark(),
		OnSubmit: func(v string) tui.Msg {
			submitted = v
			return "submitted"
		},
	})
	for _, r := range "go" {
		app.HandleInput(input.KeyEvent{Rune: r})
	}
	cmds := app.HandleInput(input.KeyEvent{Key: input.KeyEnter})
	if submitted != "go" {
		t.Errorf("submitted = %q, want %q", submitted, "go")
	}
	if len(cmds) != 1 || cmds[0]() != "submitted" {
		t.Errorf("cmds = %v, want a Cmd yielding \"submitted\"", cmds)
	}
}

func TestTextInputValueOnlyReadOnMount(t *testing.T) {
	var tr tui.Tree
	tr.Reconcile(TextInput(TextInputOptions{Theme: style.DefaultDark(), Value: "initial"}))
	buf := cell.NewBuffer(12, 3)
	tr.Paint(cell.NewPainter(buf))
	if !strings.Contains(buf.String(), "initial") {
		t.Fatalf("first paint = %q, want to contain \"initial\"", buf.String())
	}

	// A later frame passing a *different* Value must not resync the
	// buffer — TextInput is uncontrolled after mount (see its doc
	// comment).
	tr.Reconcile(TextInput(TextInputOptions{Theme: style.DefaultDark(), Value: "changed"}))
	buf2 := cell.NewBuffer(12, 3)
	tr.Paint(cell.NewPainter(buf2))
	if !strings.Contains(buf2.String(), "initial") || strings.Contains(buf2.String(), "changed") {
		t.Errorf("second paint = %q, want still \"initial\" (uncontrolled after mount)", buf2.String())
	}
}

func TestTextInputPlaceholderShownWhenEmpty(t *testing.T) {
	node := TextInput(TextInputOptions{Theme: style.DefaultDark(), Placeholder: "search..."})
	buf := cell.NewBuffer(14, 3)
	paintNode(t, node, buf)
	if !strings.Contains(buf.String(), "search...") {
		t.Errorf("Buffer = %q, want to contain placeholder", buf.String())
	}
}

func TestTextInputFocusedPlaceholderKeepsLeadingRune(t *testing.T) {
	node := TextInput(TextInputOptions{Theme: style.DefaultDark(), Placeholder: "search..."})
	tr := newTree(t, node)
	tr.Focusables()[0].SetFocused(true)
	buf := cell.NewBuffer(14, 3)
	tr.Paint(cell.NewPainter(buf))
	if !strings.Contains(buf.String(), "search...") {
		t.Errorf("Buffer = %q, want to still contain the full placeholder while focused", buf.String())
	}
}

func TestTextInputShiftArrowSelectsAndTypingReplaces(t *testing.T) {
	var value string
	app := textInputApp(t, TextInputOptions{
		Theme: style.DefaultDark(),
		OnChange: func(v string) tui.Msg {
			value = v
			return nil
		},
	})
	for _, r := range "hello" {
		app.HandleInput(input.KeyEvent{Rune: r})
	}
	// Cursor is at the end (5); Shift+Left x3 selects "llo" (offsets 2-5).
	for range 3 {
		app.HandleInput(input.KeyEvent{Key: input.KeyLeft, Mod: input.ModShift})
	}
	app.HandleInput(input.KeyEvent{Rune: 'X'})
	if value != "heX" {
		t.Fatalf("value = %q, want %q (typing over the selection should replace it)", value, "heX")
	}
}

func TestTextInputBackspaceDeletesSelection(t *testing.T) {
	var value string
	app := textInputApp(t, TextInputOptions{
		Theme: style.DefaultDark(),
		OnChange: func(v string) tui.Msg {
			value = v
			return nil
		},
	})
	for _, r := range "hello" {
		app.HandleInput(input.KeyEvent{Rune: r})
	}
	for range 3 {
		app.HandleInput(input.KeyEvent{Key: input.KeyLeft, Mod: input.ModShift})
	}
	app.HandleInput(input.KeyEvent{Key: input.KeyBackspace})
	if value != "he" {
		t.Fatalf("value = %q, want %q (Backspace should delete the whole selection, not one character)", value, "he")
	}
}

func TestTextInputNonShiftArrowCollapsesSelectionToEdge(t *testing.T) {
	var value string
	app := textInputApp(t, TextInputOptions{
		Theme: style.DefaultDark(),
		OnChange: func(v string) tui.Msg {
			value = v
			return nil
		},
	})
	for _, r := range "hello" {
		app.HandleInput(input.KeyEvent{Rune: r})
	}
	for range 3 { // selects "llo" (offsets 2-5), cursor at 2 (anchor 5)
		app.HandleInput(input.KeyEvent{Key: input.KeyLeft, Mod: input.ModShift})
	}
	// A plain (non-shift) Left with a selection active collapses to the
	// selection's start rather than moving one more rune left.
	app.HandleInput(input.KeyEvent{Key: input.KeyLeft})
	app.HandleInput(input.KeyEvent{Rune: 'X'})
	if value != "heXllo" {
		t.Fatalf("value = %q, want %q (Left should have collapsed to the selection start, offset 2, then inserted there)", value, "heXllo")
	}
}

func TestTextInputClickSetsCursorPosition(t *testing.T) {
	var value string
	app := textInputApp(t, TextInputOptions{
		Theme: style.DefaultDark(),
		OnChange: func(v string) tui.Msg {
			value = v
			return nil
		},
	})
	for _, r := range "hello" {
		app.HandleInput(input.KeyEvent{Rune: r})
	}
	// Content starts at local X=1 (X=0 is the border); clicking X=3 lands
	// on buffer offset 2 ("l", the third character).
	app.HandleInput(input.MouseEvent{X: 3, Y: 1, Button: input.MouseLeft})
	app.HandleInput(input.KeyEvent{Rune: 'X'})
	if value != "heXllo" {
		t.Fatalf("value = %q, want %q (click should have placed the cursor at offset 2)", value, "heXllo")
	}
}

func TestTextInputClickDragSelectsText(t *testing.T) {
	var value string
	app := textInputApp(t, TextInputOptions{
		Theme: style.DefaultDark(),
		OnChange: func(v string) tui.Msg {
			value = v
			return nil
		},
	})
	for _, r := range "hello" {
		app.HandleInput(input.KeyEvent{Rune: r})
	}
	app.HandleInput(input.MouseEvent{X: 1, Y: 1, Button: input.MouseLeft})             // press at offset 0
	app.HandleInput(input.MouseEvent{X: 4, Y: 1, Button: input.MouseLeft, Drag: true}) // drag to offset 3
	app.HandleInput(input.MouseEvent{X: 4, Y: 1, Button: input.MouseRelease})
	app.HandleInput(input.KeyEvent{Key: input.KeyBackspace})
	if value != "lo" {
		t.Fatalf("value = %q, want %q (drag should have selected \"hel\", offsets 0-3, for Backspace to delete)", value, "lo")
	}
}

func TestTextInputShiftClickExtendsSelection(t *testing.T) {
	var value string
	app := textInputApp(t, TextInputOptions{
		Theme: style.DefaultDark(),
		OnChange: func(v string) tui.Msg {
			value = v
			return nil
		},
	})
	for _, r := range "hello" {
		app.HandleInput(input.KeyEvent{Rune: r})
	}
	app.HandleInput(input.MouseEvent{X: 1, Y: 1, Button: input.MouseLeft}) // cursor -> offset 0
	app.HandleInput(input.MouseEvent{X: 4, Y: 1, Button: input.MouseLeft, Mod: input.ModShift})
	app.HandleInput(input.KeyEvent{Key: input.KeyBackspace})
	if value != "lo" {
		t.Fatalf("value = %q, want %q (Shift+click should extend the selection from offset 0 to 3)", value, "lo")
	}
}

func TestTextInputSelectionHighlightedInBuffer(t *testing.T) {
	app := textInputApp(t, TextInputOptions{Theme: style.DefaultDark()})
	for _, r := range "hello" {
		app.HandleInput(input.KeyEvent{Rune: r})
	}
	for range 3 {
		app.HandleInput(input.KeyEvent{Key: input.KeyLeft, Mod: input.ModShift})
	}
	// Selection is offsets 2-5 ("llo"), at local X=3..5 (content starts
	// at X=1). X=1,2 ("he") should be unstyled; X=3 should be reversed.
	if got := app.Buffer().At(1, 1).Style.Attr & cell.AttrReverse; got != 0 {
		t.Errorf("'h' (outside selection) has reverse attr set, want none")
	}
	if got := app.Buffer().At(3, 1).Style.Attr & cell.AttrReverse; got == 0 {
		t.Errorf("'l' (inside selection) has no reverse attr, want it highlighted")
	}
}

func TestTextInputPasteInsertsAndStripsNewlines(t *testing.T) {
	var value string
	app := textInputApp(t, TextInputOptions{
		Theme: style.DefaultDark(),
		OnChange: func(v string) tui.Msg {
			value = v
			return nil
		},
	})
	app.HandleInput(input.PasteEvent{Text: "ab\ncd"})
	if value != "abcd" {
		t.Errorf("value after paste = %q, want %q", value, "abcd")
	}
}
