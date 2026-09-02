package widget

import (
	"strconv"
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

func TestTextAreaPaintWideRuneNoCorruption(t *testing.T) {
	// A wide rune (width 2) followed by another character: Paint's row
	// loop must place the wide rune, its continuation cell, and the
	// next character at their true screen columns rather than assuming
	// one buffer rune is always one column — a stale 1:1 assumption
	// would have the loop's next iteration immediately overwrite the
	// continuation cell with 'b'.
	node := TextArea(TextAreaOptions{Theme: style.DefaultDark(), Value: "中b"})
	buf := cell.NewBuffer(6, 3)
	paintNode(t, node, buf)

	if got := buf.At(1, 1); got.Rune != '中' || got.Width != 2 {
		t.Errorf("At(1,1) = %+v, want the wide rune '中' with Width 2", got)
	}
	if got := buf.At(2, 1); !got.IsContinuation() {
		t.Errorf("At(2,1) = %+v, want an intact continuation cell, not overwritten by the next rune", got)
	}
	if got := buf.At(3, 1); got.Rune != 'b' {
		t.Errorf("At(3,1) = %+v, want 'b' at its true screen column (past the wide rune's 2 columns)", got)
	}
}

func TestTextAreaClickAccountsForWideRuneColumns(t *testing.T) {
	app, value := textAreaApp(t, TextAreaOptions{Theme: style.DefaultDark(), Value: "中y"})
	// Border occupies row 0/col 0; inner starts at local (1,1). '中'
	// spans screen columns 1-2, so 'y' sits at column 3 — X=3 should
	// land the cursor right before 'y', not one column short the way
	// rune-count-based (rather than screen-column-based) hit-testing
	// would place it.
	app.HandleInput(input.MouseEvent{X: 3, Y: 1, Button: input.MouseLeft})
	app.HandleInput(input.KeyEvent{Rune: 'Z'})
	if *value != "中Zy" {
		t.Fatalf("value = %q, want %q (click should account for the wide rune's 2-column width)", *value, "中Zy")
	}
}

func TestTextAreaMoveVerticalPreservesVisualColumnAcrossWideRunes(t *testing.T) {
	app, value := textAreaApp(t, TextAreaOptions{Theme: style.DefaultDark(), Value: "中abc\nxyz"})
	app.HandleInput(input.KeyEvent{Key: input.KeyHome, Mod: input.ModCtrl}) // cursor -> 0
	app.HandleInput(input.KeyEvent{Key: input.KeyRight})
	app.HandleInput(input.KeyEvent{Key: input.KeyRight}) // cursor -> right after "中a" (visual col 3)
	app.HandleInput(input.KeyEvent{Key: input.KeyDown})  // -> visual col 3 on "xyz": past 'z', not one short
	app.HandleInput(input.KeyEvent{Rune: 'Z'})
	if *value != "中abc\nxyzZ" {
		t.Fatalf("value = %q, want %q (Down should preserve visual column, not buffer rune count, across the wide rune on the line above)", *value, "中abc\nxyzZ")
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

func TestTextAreaCtrlHomeEndJumpToBufferStartAndEnd(t *testing.T) {
	app, value := textAreaApp(t, TextAreaOptions{Theme: style.DefaultDark(), Value: "abcd\nxy\nefgh"})
	// Cursor mounts at the very end. Ctrl+Home should jump all the way
	// to offset 0 regardless of line, unlike plain Home (start of the
	// current line only).
	app.HandleInput(input.KeyEvent{Key: input.KeyHome, Mod: input.ModCtrl})
	app.HandleInput(input.KeyEvent{Rune: 'Z'})
	if *value != "Zabcd\nxy\nefgh" {
		t.Fatalf("value after Ctrl+Home = %q, want %q", *value, "Zabcd\nxy\nefgh")
	}

	app.HandleInput(input.KeyEvent{Key: input.KeyEnd, Mod: input.ModCtrl})
	app.HandleInput(input.KeyEvent{Rune: 'Q'})
	if *value != "Zabcd\nxy\nefghQ" {
		t.Fatalf("value after Ctrl+End = %q, want %q", *value, "Zabcd\nxy\nefghQ")
	}
}

func TestTextAreaCtrlLeftRightJumpsWordBoundaries(t *testing.T) {
	app, value := textAreaApp(t, TextAreaOptions{Theme: style.DefaultDark(), Value: "foo bar"})
	app.HandleInput(input.KeyEvent{Key: input.KeyHome})

	// From the start, Ctrl+Right should skip the current word ("foo")
	// and land right after it, at the following space.
	app.HandleInput(input.KeyEvent{Key: input.KeyRight, Mod: input.ModCtrl})
	app.HandleInput(input.KeyEvent{Rune: 'X'})
	if *value != "fooX bar" {
		t.Fatalf("value after first Ctrl+Right = %q, want %q", *value, "fooX bar")
	}

	// From just after "fooX", Ctrl+Right should skip the space and then
	// all of "bar", landing at the buffer's end.
	app.HandleInput(input.KeyEvent{Key: input.KeyRight, Mod: input.ModCtrl})
	app.HandleInput(input.KeyEvent{Rune: 'Y'})
	if *value != "fooX barY" {
		t.Fatalf("value after second Ctrl+Right = %q, want %q", *value, "fooX barY")
	}

	// From the end, Ctrl+Left should land right before the last word
	// ("bar"), skipping past "Y" only because "Y" is itself a word
	// character contiguous with "bar" (no space separates them).
	app.HandleInput(input.KeyEvent{Key: input.KeyLeft, Mod: input.ModCtrl})
	app.HandleInput(input.KeyEvent{Rune: 'Z'})
	if *value != "fooX ZbarY" {
		t.Fatalf("value after Ctrl+Left = %q, want %q", *value, "fooX ZbarY")
	}
}

func TestTextAreaCtrlShiftRightSelectsWord(t *testing.T) {
	app, value := textAreaApp(t, TextAreaOptions{Theme: style.DefaultDark(), Value: "foo bar"})
	app.HandleInput(input.KeyEvent{Key: input.KeyHome})
	app.HandleInput(input.KeyEvent{Key: input.KeyRight, Mod: input.ModCtrl | input.ModShift})
	app.HandleInput(input.KeyEvent{Rune: 'X'})
	if *value != "X bar" {
		t.Fatalf("value = %q, want %q (Ctrl+Shift+Right should select \"foo\" for typing to replace)", *value, "X bar")
	}
}

func TestTextAreaPageDownPageUpMoveByVisibleLineCount(t *testing.T) {
	// App height 6 minus the 2-row border leaves an inner height of 4
	// visible lines — see textArea's Paint.
	app, value := textAreaApp(t, TextAreaOptions{
		Theme: style.DefaultDark(),
		Value: "0\n1\n2\n3\n4\n5\n6\n7\n8\n9",
	})

	app.HandleInput(input.KeyEvent{Key: input.KeyHome, Mod: input.ModCtrl}) // -> offset 0, line "0"
	app.HandleInput(input.KeyEvent{Key: input.KeyPgDown})                   // -> down 4 lines, to line "4"
	app.HandleInput(input.KeyEvent{Rune: 'X'})
	want := "0\n1\n2\n3\nX4\n5\n6\n7\n8\n9"
	if *value != want {
		t.Fatalf("value after PgDown = %q, want %q", *value, want)
	}

	app.HandleInput(input.KeyEvent{Key: input.KeyPgUp}) // -> up 4 lines, back to line "0"
	app.HandleInput(input.KeyEvent{Rune: 'Y'})
	want = "0Y\n1\n2\n3\nX4\n5\n6\n7\n8\n9"
	if *value != want {
		t.Fatalf("value after PgUp = %q, want %q", *value, want)
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

func TestTextAreaInitialCursorPlacesCursorAtOffset(t *testing.T) {
	cursor := 3
	app, value := textAreaApp(t, TextAreaOptions{Theme: style.DefaultDark(), Value: "abcdef", InitialCursor: &cursor})
	app.HandleInput(input.KeyEvent{Rune: 'X'})
	if *value != "abcXdef" {
		t.Fatalf("value = %q, want %q (typed char should land at InitialCursor's offset)", *value, "abcXdef")
	}
}

func TestTextAreaInitialCursorNilKeepsEndOfTextDefault(t *testing.T) {
	app, value := textAreaApp(t, TextAreaOptions{Theme: style.DefaultDark(), Value: "abc"})
	app.HandleInput(input.KeyEvent{Rune: 'X'})
	if *value != "abcX" {
		t.Fatalf("value = %q, want %q (nil InitialCursor must keep the old end-of-text default)", *value, "abcX")
	}
}

func TestTextAreaInitialCursorClampsPastEnd(t *testing.T) {
	cursor := 100
	app, value := textAreaApp(t, TextAreaOptions{Theme: style.DefaultDark(), Value: "abc", InitialCursor: &cursor})
	app.HandleInput(input.KeyEvent{Rune: 'X'})
	if *value != "abcX" {
		t.Fatalf("value = %q, want %q (out-of-range InitialCursor should clamp to len(Value))", *value, "abcX")
	}
}

func TestTextAreaOnCursorChangeFiresOnNavigationWithoutOnChange(t *testing.T) {
	var offsets []int
	m := &widgetHostModel{node: TextArea(TextAreaOptions{
		Theme: style.DefaultDark(),
		Value: "abc",
		OnChange: func(v string) tui.Msg {
			t.Fatalf("OnChange fired for pure cursor navigation, value = %q", v)
			return nil
		},
		OnCursorChange: func(offset int) tui.Msg {
			offsets = append(offsets, offset)
			return nil
		},
	})}
	app := tui.NewApp(m, 14, 6)

	app.HandleInput(input.KeyEvent{Key: input.KeyLeft})
	app.HandleInput(input.KeyEvent{Key: input.KeyLeft})
	app.HandleInput(input.KeyEvent{Key: input.KeyHome}) // -> offset 0
	app.HandleInput(input.KeyEvent{Key: input.KeyHome}) // already at line start: no-op, no callback

	if want := []int{2, 1, 0}; !equalInts(offsets, want) {
		t.Fatalf("offsets = %v, want %v", offsets, want)
	}
}

func TestTextAreaOnCursorChangeFiresOnEditAlongsideOnChange(t *testing.T) {
	var value string
	var offset int
	m := &widgetHostModel{node: TextArea(TextAreaOptions{
		Theme: style.DefaultDark(),
		OnChange: func(v string) tui.Msg {
			value = v
			return nil
		},
		OnCursorChange: func(o int) tui.Msg {
			offset = o
			return nil
		},
	})}
	app := tui.NewApp(m, 14, 6)

	app.HandleInput(input.KeyEvent{Rune: 'a'})
	app.HandleInput(input.KeyEvent{Rune: 'b'})

	if value != "ab" {
		t.Fatalf("value = %q, want %q", value, "ab")
	}
	if offset != 2 {
		t.Fatalf("offset = %d, want %d", offset, 2)
	}
}

func TestTextAreaOnCursorChangeFiresOnMouseClick(t *testing.T) {
	var offset = -1
	m := &widgetHostModel{node: TextArea(TextAreaOptions{
		Theme: style.DefaultDark(),
		Value: "abc\ndef",
		OnCursorChange: func(o int) tui.Msg {
			offset = o
			return nil
		},
	})}
	app := tui.NewApp(m, 14, 6)

	app.HandleInput(input.MouseEvent{X: 2, Y: 2, Button: input.MouseLeft}) // second line ("def"), col 1 -> offset 5

	if offset != 5 {
		t.Fatalf("offset = %d, want %d", offset, 5)
	}
}

func TestTextAreaOnCursorChangeBatchesWithOnChangeCmd(t *testing.T) {
	m := &widgetHostModel{node: TextArea(TextAreaOptions{
		Theme:          style.DefaultDark(),
		OnChange:       func(v string) tui.Msg { return "changed:" + v },
		OnCursorChange: func(o int) tui.Msg { return "cursor" },
	})}
	app := tui.NewApp(m, 14, 6)

	cmds := app.HandleInput(input.KeyEvent{Rune: 'a'})
	if len(cmds) != 1 {
		t.Fatalf("cmds = %v, want a single (batched) Cmd", cmds)
	}
	batch, ok := cmds[0]().(tui.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("cmds[0]() = %#v, want a 2-element tui.BatchMsg", cmds[0]())
	}
	got := []tui.Msg{batch[0](), batch[1]()}
	want := []tui.Msg{"changed:a", "cursor"}
	if got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("batch msgs = %v, want %v", got, want)
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestTextAreaGutterShowsLineNumbersAndShiftsContent(t *testing.T) {
	node := TextArea(TextAreaOptions{
		Theme: style.DefaultDark(),
		Value: "one\ntwo\nthree",
		Gutter: func(lineIdx int) (string, cell.Style) {
			return strconv.Itoa(lineIdx + 1), style.DefaultDark().MutedText()
		},
	})
	buf := cell.NewBuffer(12, 6)
	paintNode(t, node, buf)

	// Single-digit line numbers: gutter column is 2 wide (1 digit + 1
	// separator), so the digit sits at inner-relative column 0 (absolute
	// x=1) and content starts at absolute x=3.
	for row, want := range map[int]rune{1: '1', 2: '2', 3: '3'} {
		if got := buf.At(1, row).Rune; got != want {
			t.Errorf("At(1,%d) = %q, want gutter digit %q", row, got, want)
		}
	}
	if got := buf.At(3, 1).Rune; got != 'o' {
		t.Errorf("At(3,1) = %q, want content 'o' (of \"one\") shifted past the gutter column", got)
	}
	if got := buf.At(3, 2).Rune; got != 't' {
		t.Errorf("At(3,2) = %q, want content 't' (of \"two\") shifted past the gutter column", got)
	}
}

func TestTextAreaGutterRightAlignsAndPadsToWidestVisibleRow(t *testing.T) {
	node := TextArea(TextAreaOptions{
		Theme: style.DefaultDark(),
		Value: "a\nb",
		Gutter: func(lineIdx int) (string, cell.Style) {
			if lineIdx == 0 {
				return "1", style.DefaultDark().MutedText()
			}
			return "22", style.DefaultDark().MutedText()
		},
	})
	buf := cell.NewBuffer(12, 6)
	paintNode(t, node, buf)

	// Widest visible row is "22" (width 2), so the gutter column is 3
	// wide (2 text + 1 separator): "1" pads left with a space to fill
	// the 2-wide text field, "22" fills it exactly, and both rows share
	// one blank separator column before content.
	if got := buf.At(1, 1).Rune; got != ' ' {
		t.Errorf("At(1,1) = %q, want left-pad space before the short \"1\"", got)
	}
	if got := buf.At(2, 1).Rune; got != '1' {
		t.Errorf("At(2,1) = %q, want '1' right-aligned in the gutter column", got)
	}
	if got := buf.At(3, 1).Rune; got != ' ' {
		t.Errorf("At(3,1) = %q, want the separator column blank", got)
	}
	if got := buf.At(4, 1).Rune; got != 'a' {
		t.Errorf("At(4,1) = %q, want content 'a' past the gutter", got)
	}

	if got := buf.At(1, 2).Rune; got != '2' {
		t.Errorf("At(1,2) = %q, want '2' (first digit of \"22\", no padding needed)", got)
	}
	if got := buf.At(2, 2).Rune; got != '2' {
		t.Errorf("At(2,2) = %q, want '2' (second digit of \"22\")", got)
	}
	if got := buf.At(4, 2).Rune; got != 'b' {
		t.Errorf("At(4,2) = %q, want content 'b' past the gutter", got)
	}
}

func TestTextAreaGutterShownWhenBufferEmpty(t *testing.T) {
	node := TextArea(TextAreaOptions{
		Theme:       style.DefaultDark(),
		Placeholder: "type here",
		Gutter: func(lineIdx int) (string, cell.Style) {
			return "1", style.DefaultDark().MutedText()
		},
	})
	buf := cell.NewBuffer(14, 5)
	paintNode(t, node, buf)

	if got := buf.At(1, 1).Rune; got != '1' {
		t.Errorf("At(1,1) = %q, want the gutter digit for the (empty) first line", got)
	}
	if got := buf.At(3, 1).Rune; got != 't' {
		t.Errorf("At(3,1) = %q, want placeholder text shifted past the gutter column", got)
	}
}

func TestTextAreaNilGutterAddsNoColumn(t *testing.T) {
	node := TextArea(TextAreaOptions{Theme: style.DefaultDark(), Value: "one"})
	buf := cell.NewBuffer(12, 5)
	paintNode(t, node, buf)

	if got := buf.At(1, 1).Rune; got != 'o' {
		t.Errorf("At(1,1) = %q, want content starting right after the border with no Gutter set", got)
	}
}
