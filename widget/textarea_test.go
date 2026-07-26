package widget

import (
	"strings"
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

func textAreaApp(t *testing.T, opts TextAreaOptions) (*tui.App, *string) {
	t.Helper()
	var value string
	opts.OnChange = func(v string) tui.Msg {
		value = v
		return nil
	}
	m := &widgetHostModel{node: TextArea(opts)}
	return tui.NewApp(m, 14, 6), &value
}

func TestTextAreaEnterInsertsNewline(t *testing.T) {
	app, value := textAreaApp(t, TextAreaOptions{Theme: style.DefaultDark()})
	for _, r := range "ab" {
		app.HandleInput(input.KeyEvent{Rune: r})
	}
	app.HandleInput(input.KeyEvent{Key: input.KeyEnter})
	for _, r := range "cd" {
		app.HandleInput(input.KeyEvent{Rune: r})
	}
	if *value != "ab\ncd" {
		t.Fatalf("value = %q, want %q", *value, "ab\ncd")
	}
}

func TestTextAreaCtrlEnterSubmitsInsteadOfNewline(t *testing.T) {
	var submitted string
	var changed string
	m := &widgetHostModel{node: TextArea(TextAreaOptions{
		Theme: style.DefaultDark(),
		OnChange: func(v string) tui.Msg {
			changed = v
			return nil
		},
		OnSubmit: func(v string) tui.Msg {
			submitted = v
			return "sent"
		},
	})}
	app := tui.NewApp(m, 14, 6)
	for _, r := range "hi" {
		app.HandleInput(input.KeyEvent{Rune: r})
	}
	cmds := app.HandleInput(input.KeyEvent{Key: input.KeyEnter, Mod: input.ModCtrl})

	if submitted != "hi" {
		t.Errorf("submitted = %q, want %q", submitted, "hi")
	}
	if changed != "hi" {
		t.Errorf("changed = %q, want %q (Ctrl+Enter must not have inserted a newline)", changed, "hi")
	}
	if len(cmds) != 1 || cmds[0]() != "sent" {
		t.Errorf("cmds = %v, want a Cmd yielding \"sent\"", cmds)
	}
}

func TestTextAreaUpDownPreservesColumn(t *testing.T) {
	app, value := textAreaApp(t, TextAreaOptions{Theme: style.DefaultDark(), Value: "abcd\nxy\nefgh"})
	_ = value

	// Cursor mounts at the end (line 2, col 4, end of "efgh"). Move up
	// twice to land on the short middle line, then up once more onto
	// the first line — column should clamp to "xy"'s length (2) while
	// on line 1, then jump back out to column 4 on line 0 since it's
	// long enough.
	app.HandleInput(input.KeyEvent{Key: input.KeyUp}) // -> line 1 ("xy"), col clamped to 2
	app.HandleInput(input.KeyEvent{Key: input.KeyBackspace})
	if *value != "abcd\nx\nefgh" {
		t.Fatalf("value after Up+Backspace = %q, want %q (col should have clamped to end of \"xy\")", *value, "abcd\nx\nefgh")
	}
}

func TestTextAreaPasteAllowsNewlines(t *testing.T) {
	app, value := textAreaApp(t, TextAreaOptions{Theme: style.DefaultDark()})
	app.HandleInput(input.PasteEvent{Text: "line1\nline2"})
	if *value != "line1\nline2" {
		t.Errorf("value after paste = %q, want %q (TextArea allows pasted newlines, unlike TextInput)", *value, "line1\nline2")
	}
}

func TestTextAreaUndoRedo(t *testing.T) {
	app, value := textAreaApp(t, TextAreaOptions{Theme: style.DefaultDark()})
	app.HandleInput(input.KeyEvent{Rune: 'a'})
	app.HandleInput(input.KeyEvent{Key: input.KeyEnter})
	app.HandleInput(input.KeyEvent{Rune: 'b'})
	if *value != "a\nb" {
		t.Fatalf("value = %q, want %q", *value, "a\nb")
	}
	app.HandleInput(input.KeyEvent{Rune: 'z', Mod: input.ModCtrl})
	if *value != "a\n" {
		t.Fatalf("value after undo = %q, want %q", *value, "a\n")
	}
	app.HandleInput(input.KeyEvent{Rune: 'y', Mod: input.ModCtrl})
	if *value != "a\nb" {
		t.Fatalf("value after redo = %q, want %q", *value, "a\nb")
	}
}

func TestTextAreaPaintShowsMultipleLines(t *testing.T) {
	node := TextArea(TextAreaOptions{Theme: style.DefaultDark(), Value: "one\ntwo\nthree"})
	buf := cell.NewBuffer(10, 5)
	paintNode(t, node, buf)

	got := buf.String()
	for _, want := range []string{"one", "two", "three"} {
		if !strings.Contains(got, want) {
			t.Errorf("Buffer missing line %q:\n%s", want, got)
		}
	}
}

func TestTextAreaPlaceholderShownWhenEmpty(t *testing.T) {
	node := TextArea(TextAreaOptions{Theme: style.DefaultDark(), Placeholder: "type here"})
	buf := cell.NewBuffer(14, 5)
	paintNode(t, node, buf)
	if !strings.Contains(buf.String(), "type here") {
		t.Errorf("Buffer = %q, want to contain placeholder", buf.String())
	}
}
