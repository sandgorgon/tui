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
