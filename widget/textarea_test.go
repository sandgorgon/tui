package widget

import (
	"strings"
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/layout"
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

func TestTextAreaTabInsertsLiteralTabCharacter(t *testing.T) {
	app, value := textAreaApp(t, TextAreaOptions{Theme: style.DefaultDark()})
	app.HandleInput(input.KeyEvent{Rune: 'a'})
	app.HandleInput(input.KeyEvent{Key: input.KeyTab})
	app.HandleInput(input.KeyEvent{Rune: 'b'})

	if *value != "a\tb" {
		t.Errorf("value = %q, want %q (Tab should insert a literal tab, not move focus)", *value, "a\tb")
	}
}

// textAreaAndFocusable builds a two-widget frame — a TextArea and a
// second Focusable widget forwarding a tagged Msg — for tests that
// need to check whether a key moved focus away from the TextArea.
func textAreaAndFocusable(opts TextAreaOptions) (*tui.App, *string) {
	var value string
	opts.OnChange = func(v string) tui.Msg {
		value = v
		return nil
	}
	m := &widgetHostModel{node: tui.Box(layout.Horizontal,
		tui.Child(layout.Fill(1), TextArea(opts)),
		tui.Child(layout.Fill(1), tui.Focusable("other", tui.Text("other", cell.Style{}), func(e input.Event) tui.Msg { return "other" })),
	)}
	return tui.NewApp(m, 30, 8), &value
}

func TestTextAreaEscReleasesFocusByDefault(t *testing.T) {
	app, value := textAreaAndFocusable(TextAreaOptions{Theme: style.DefaultDark()})

	app.HandleInput(input.KeyEvent{Key: input.KeyEsc})
	if *value != "" {
		t.Errorf("value = %q, want empty (Esc should release focus, not be typed into the field)", *value)
	}

	// Focus should now be on "other": pressing a key should produce
	// its tagged Cmd, not reach the TextArea.
	cmds := app.HandleInput(input.KeyEvent{Rune: 'x'})
	if len(cmds) != 1 || cmds[0]() != "other" {
		t.Fatalf("expected focus to have moved to the other widget after Esc, got cmds=%v", cmds)
	}
}

func TestTextAreaCustomReleaseKey(t *testing.T) {
	app, value := textAreaAndFocusable(TextAreaOptions{
		Theme:      style.DefaultDark(),
		ReleaseKey: input.KeyEvent{Rune: 'q', Mod: input.ModCtrl},
	})

	// Esc is no longer the release key for this instance, so it must
	// be handled by the field itself (a no-op key for TextArea) rather
	// than releasing focus.
	app.HandleInput(input.KeyEvent{Key: input.KeyEsc})
	if cmds := app.HandleInput(input.KeyEvent{Rune: 'a'}); len(cmds) != 0 {
		t.Fatalf("expected the field still focused after Esc (not the release key here), got cmds=%v", cmds)
	}
	if *value != "a" {
		t.Fatalf("value = %q, want %q (still typing into the field)", *value, "a")
	}

	// The release key itself just moves focus (like Tab) and returns no
	// Cmd for that event — check the *next* keystroke lands on "other".
	if cmds := app.HandleInput(input.KeyEvent{Rune: 'q', Mod: input.ModCtrl}); len(cmds) != 0 {
		t.Fatalf("release key itself should produce no Cmd, got %v", cmds)
	}
	cmds := app.HandleInput(input.KeyEvent{Rune: 'x'})
	if len(cmds) != 1 || cmds[0]() != "other" {
		t.Fatalf("expected focus moved to \"other\" after the custom release key (Ctrl+Q), got cmds=%v", cmds)
	}
}

func TestTextAreaShiftDownSelectsAcrossLinesAndTypingReplaces(t *testing.T) {
	app, value := textAreaApp(t, TextAreaOptions{Theme: style.DefaultDark(), Value: "abcd\nxy\nefgh"})
	// Cursor mounts at the end ("efgh"'s last char, offset 11). Move to
	// the very start, then Shift+Down x2 to select everything up to the
	// equivalent column two lines down.
	app.HandleInput(input.KeyEvent{Key: input.KeyHome})
	app.HandleInput(input.KeyEvent{Key: input.KeyUp})
	app.HandleInput(input.KeyEvent{Key: input.KeyUp})
	app.HandleInput(input.KeyEvent{Key: input.KeyHome}) // now at offset 0, start of "abcd"
	app.HandleInput(input.KeyEvent{Key: input.KeyDown, Mod: input.ModShift})
	app.HandleInput(input.KeyEvent{Key: input.KeyDown, Mod: input.ModShift})
	app.HandleInput(input.KeyEvent{Rune: 'X'})
	if *value != "Xefgh" {
		t.Fatalf("value = %q, want %q (typing should replace the selection spanning \"abcd\\nxy\\n\")", *value, "Xefgh")
	}
}

func TestTextAreaBackspaceDeletesSelectionAcrossNewline(t *testing.T) {
	app, value := textAreaApp(t, TextAreaOptions{Theme: style.DefaultDark(), Value: "abc\ndef"})
	app.HandleInput(input.KeyEvent{Key: input.KeyHome}) // still on line 1 ("def"), col 0
	app.HandleInput(input.KeyEvent{Key: input.KeyUp})   // -> line 0 ("abc"), col 0
	app.HandleInput(input.KeyEvent{Key: input.KeyRight, Mod: input.ModShift})
	app.HandleInput(input.KeyEvent{Key: input.KeyDown, Mod: input.ModShift}) // extends to "abc\nd"
	app.HandleInput(input.KeyEvent{Key: input.KeyBackspace})
	if *value != "ef" {
		t.Fatalf("value = %q, want %q (Backspace should delete the selection spanning the newline, joining the remaining halves)", *value, "ef")
	}
}

func TestTextAreaClickSetsCursorOnCorrectLine(t *testing.T) {
	app, value := textAreaApp(t, TextAreaOptions{Theme: style.DefaultDark(), Value: "abcd\nxy\nefgh"})
	// Border occupies row 0/col 0; content starts at local (1,1). Line 1
	// ("xy") is the second content row, so Y=2; X=2 lands on col 1 of
	// that line (offset 5+1=6, since "abcd\n" is 5 runes).
	app.HandleInput(input.MouseEvent{X: 2, Y: 2, Button: input.MouseLeft})
	app.HandleInput(input.KeyEvent{Rune: 'Z'})
	if *value != "abcd\nxZy\nefgh" {
		t.Fatalf("value = %q, want %q (click should have placed the cursor within \"xy\")", *value, "abcd\nxZy\nefgh")
	}
}

func TestTextAreaDragSelectsAcrossLines(t *testing.T) {
	app, value := textAreaApp(t, TextAreaOptions{Theme: style.DefaultDark(), Value: "abcd\nxy\nefgh"})
	// Press at start of line 0 ("abcd", Y=1,X=1 -> offset 0), drag down
	// to line 2 ("efgh", Y=3,X=1 -> offset 8, start of "efgh").
	app.HandleInput(input.MouseEvent{X: 1, Y: 1, Button: input.MouseLeft})
	app.HandleInput(input.MouseEvent{X: 1, Y: 3, Button: input.MouseLeft, Drag: true})
	app.HandleInput(input.MouseEvent{X: 1, Y: 3, Button: input.MouseRelease})
	app.HandleInput(input.KeyEvent{Key: input.KeyBackspace})
	if *value != "efgh" {
		t.Fatalf("value = %q, want %q (drag should have selected \"abcd\\nxy\\n\" for Backspace to delete)", *value, "efgh")
	}
}

func TestTextAreaSelectionHighlightedInBuffer(t *testing.T) {
	app, _ := textAreaApp(t, TextAreaOptions{Theme: style.DefaultDark(), Value: "abcd\nxy\nefgh"})
	app.HandleInput(input.KeyEvent{Key: input.KeyHome})
	app.HandleInput(input.KeyEvent{Key: input.KeyUp})
	app.HandleInput(input.KeyEvent{Key: input.KeyUp})
	app.HandleInput(input.KeyEvent{Key: input.KeyHome}) // offset 0
	app.HandleInput(input.KeyEvent{Key: input.KeyDown, Mod: input.ModShift})

	// Line 0 ("abcd") is fully swept (selection continues into line 1),
	// so its whole row — including the padding past "abcd" — should be
	// highlighted; line 1 ("xy") isn't touched at all yet.
	if got := app.Buffer().At(1, 1).Style.Attr & cell.AttrReverse; got == 0 {
		t.Errorf("'a' on the fully-selected first line has no reverse attr, want it highlighted")
	}
	if got := app.Buffer().At(6, 1).Style.Attr & cell.AttrReverse; got == 0 {
		t.Errorf("padding past \"abcd\" on the fully-selected first line has no reverse attr, want the row highlighted to show the selection continues")
	}
	// X=1 ("x") is where the cursor itself now sits (the selection's
	// moving end, at line 1's start) — legitimately highlighted as the
	// caret, not as "inside" the selection. X=2 ("y") has neither and
	// should be plain.
	if got := app.Buffer().At(2, 2).Style.Attr & cell.AttrReverse; got != 0 {
		t.Errorf("'y' on the untouched second line has reverse attr set, want none")
	}
}

func TestTextAreaHighlightsOverrideBaseStyle(t *testing.T) {
	red := cell.Style{Fg: cell.RGBColor(255, 0, 0)}
	app, _ := textAreaApp(t, TextAreaOptions{
		Theme: style.DefaultDark(), Value: "abcd\nxy",
		Highlights: []StyleSpan{{Start: 2, End: 4, Style: red}}, // "cd"
	})

	if got := app.Buffer().At(2, 1).Style.Fg; got != cell.DefaultColor() {
		t.Errorf("'b' (offset 1, outside the span) Fg = %v, want the theme default", got)
	}
	if got := app.Buffer().At(4, 1).Style.Fg; got != red.Fg {
		t.Errorf("'d' (offset 3, inside the span) Fg = %v, want %v", got, red.Fg)
	}
	if got := app.Buffer().At(2, 2).Style.Fg; got != cell.DefaultColor() {
		t.Errorf("'y' (line 1, past the span's End) Fg = %v, want the theme default", got)
	}
}

func TestTextAreaHighlightsComposeWithSelection(t *testing.T) {
	red := cell.Style{Fg: cell.RGBColor(255, 0, 0)}
	app, _ := textAreaApp(t, TextAreaOptions{
		Theme: style.DefaultDark(), Value: "abcd",
		Highlights: []StyleSpan{{Start: 2, End: 4, Style: red}}, // "cd"
	})
	app.HandleInput(input.KeyEvent{Key: input.KeyHome})
	app.HandleInput(input.KeyEvent{Key: input.KeyEnd, Mod: input.ModShift}) // select "abcd"

	c := app.Buffer().At(4, 1) // 'd', inside both the span and the selection
	if c.Style.Fg != red.Fg {
		t.Errorf("Fg = %v, want the span's color preserved under the selection highlight", c.Style.Fg)
	}
	if c.Style.Attr&cell.AttrReverse == 0 {
		t.Errorf("Attr = %v, want AttrReverse still applied for the selection on top of the span", c.Style.Attr)
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
