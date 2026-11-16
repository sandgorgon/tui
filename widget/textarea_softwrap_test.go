package widget

import (
	"strconv"
	"strings"
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

func TestWrapLine(t *testing.T) {
	cases := []struct {
		name, line string
		width      int
		want       []string
	}{
		{"breaks after the last space that fits", "aaa bbb ccc", 6, []string{"aaa ", "bbb ", "ccc"}},
		{"a word wider than the row is broken mid-word", "abcdefghij", 6, []string{"abcde", "fghij"}},
		{"a space that doesn't fit hangs in the last column", "abcde fg", 6, []string{"abcde ", "fg"}},
		{"a hanging space at line end leaves an empty row for the cursor", "abcde ", 6, []string{"abcde ", ""}},
		{"a line that fits is one row", "abc", 6, []string{"abc"}},
		{"an empty line is one empty row", "", 6, []string{""}},
		{"wide runes are measured in columns", "中中中", 6, []string{"中中", "中"}},
		{"width 1 still progresses and leaves room for the end cursor", "ab", 1, []string{"a", "b", ""}},
		{"a wide rune wider than the row still progresses", "中a", 1, []string{"中", "a", ""}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			buf := []rune(c.line)
			var got []string
			for _, seg := range wrapLine(buf, textLine{0, len(buf)}, c.width) {
				got = append(got, string(buf[seg.start:seg.end]))
			}
			if strings.Join(got, "|") != strings.Join(c.want, "|") {
				t.Errorf("wrapLine(%q, %d) = %q, want %q", c.line, c.width, got, c.want)
			}
		})
	}
}

// softWrapApp hosts a frameless, soft-wrapping TextArea in a w x h App,
// so screen coordinates are text coordinates (no border to offset).
func softWrapApp(t *testing.T, opts TextAreaOptions, w, h int) (*tui.App, *string) {
	t.Helper()
	var value string
	opts.Theme = style.DefaultDark()
	opts.Frameless = true
	opts.SoftWrap = true
	opts.OnChange = func(v string) tui.Msg {
		value = v
		return nil
	}
	return tui.NewApp(&widgetHostModel{node: TextArea(opts)}, w, h), &value
}

func screenRows(app *tui.App) []string {
	rows := strings.Split(app.Buffer().String(), "\n")
	for i := range rows {
		rows[i] = strings.TrimRight(rows[i], " ")
	}
	return rows
}

func typeRune(app *tui.App, r rune) { app.HandleInput(input.KeyEvent{Rune: r}) }

func TestTextAreaSoftWrapPaintsWrappedRows(t *testing.T) {
	app, _ := softWrapApp(t, TextAreaOptions{Value: "aaa bbb ccc\nxy"}, 6, 5)
	got := screenRows(app)[:4]
	want := []string{"aaa", "bbb", "ccc", "xy"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("rows = %q, want %q", got, want)
	}
}

func TestTextAreaSoftWrapCanBeToggledBetweenFrames(t *testing.T) {
	opts := TextAreaOptions{Theme: style.DefaultDark(), Value: "aaa bbb ccc", Frameless: true}
	m := &widgetHostModel{node: TextArea(opts)}
	app := tui.NewApp(m, 6, 3)
	if rows := screenRows(app); rows[1] != "" {
		t.Fatalf("unwrapped: rows = %q, want one horizontally scrolled row", rows)
	}

	opts.SoftWrap = true
	m.node = TextArea(opts)
	app.Dispatch("toggle")
	if rows := screenRows(app); rows[0] != "aaa" || rows[2] != "ccc" {
		t.Errorf("wrapped: rows = %q, want the line wrapped across three rows", rows)
	}

	opts.SoftWrap = false
	m.node = TextArea(opts)
	app.Dispatch("toggle")
	if rows := screenRows(app); rows[1] != "" {
		t.Errorf("unwrapped again: rows = %q, want one row", rows)
	}
}

func TestTextAreaSoftWrapCursorOnBoundaryDrawsOnNextRow(t *testing.T) {
	at := 4 // start of "bbb", which is also the end offset of row 0
	app, _ := softWrapApp(t, TextAreaOptions{Value: "aaa bbb", InitialCursor: &at}, 6, 3)
	if app.Buffer().At(0, 1).Style.Attr&cell.AttrReverse == 0 {
		t.Error("cursor not drawn at the start of row 1")
	}
	if app.Buffer().At(4, 0).Style.Attr&cell.AttrReverse != 0 {
		t.Error("cursor also drawn past the end of row 0")
	}
}

func TestTextAreaSoftWrapEndOfLineCursorVisible(t *testing.T) {
	// "abcde" fills the 5 content columns of a 6-wide row; the cursor
	// at its end needs the reserved sixth column.
	app, _ := softWrapApp(t, TextAreaOptions{Value: "abcde"}, 6, 2)
	if app.Buffer().At(5, 0).Style.Attr&cell.AttrReverse == 0 {
		t.Error("end-of-line cursor not drawn in the reserved last column")
	}
}

func TestTextAreaSoftWrapUpDownMoveByScreenRow(t *testing.T) {
	app, value := softWrapApp(t, TextAreaOptions{Value: "aaa bbb ccc\nxy"}, 6, 5)
	app.HandleInput(input.KeyEvent{Key: input.KeyHome, Mod: input.ModCtrl})
	app.HandleInput(input.KeyEvent{Key: input.KeyRight})
	app.HandleInput(input.KeyEvent{Key: input.KeyDown}) // row "bbb ", col 1
	typeRune(app, 'Z')
	if want := "aaa bZbb ccc\nxy"; *value != want {
		t.Fatalf("after Down: value = %q, want %q", *value, want)
	}
	app.HandleInput(input.KeyEvent{Key: input.KeyDown}) // row "ccc", col 2
	app.HandleInput(input.KeyEvent{Key: input.KeyDown}) // row "xy", col 2 (end)
	typeRune(app, 'Y')
	if want := "aaa bZbb ccc\nxyY"; *value != want {
		t.Fatalf("after Down x2: value = %q, want %q", *value, want)
	}
	app.HandleInput(input.KeyEvent{Key: input.KeyUp}) // row "ccc", col 3 (end of line)
	typeRune(app, 'X')
	if want := "aaa bZbb cccX\nxyY"; *value != want {
		t.Fatalf("after Up: value = %q, want %q", *value, want)
	}
}

func TestTextAreaSoftWrapUpClampsToRowsLastRune(t *testing.T) {
	// Rows "ab " and "cdefg". From the end of "cdefg" (col 5), Up must
	// stop on row 0's last rune (the space), not its end offset 3 —
	// that's where row 1 starts.
	app, value := softWrapApp(t, TextAreaOptions{Value: "ab cdefg"}, 6, 3)
	app.HandleInput(input.KeyEvent{Key: input.KeyUp})
	typeRune(app, 'Z')
	if want := "abZ cdefg"; *value != want {
		t.Fatalf("value = %q, want %q", *value, want)
	}
}

func TestTextAreaSoftWrapHomeEndAreWholeLine(t *testing.T) {
	app, value := softWrapApp(t, TextAreaOptions{Value: "aaa bbb ccc"}, 6, 4)
	app.HandleInput(input.KeyEvent{Key: input.KeyUp}) // onto row "bbb "
	app.HandleInput(input.KeyEvent{Key: input.KeyHome})
	typeRune(app, 'H')
	app.HandleInput(input.KeyEvent{Key: input.KeyEnd})
	typeRune(app, 'E')
	if want := "Haaa bbb cccE"; *value != want {
		t.Fatalf("value = %q, want %q (Home/End go to the buffer line's ends)", *value, want)
	}
}

func TestTextAreaSoftWrapPageMovesByScreenRows(t *testing.T) {
	app, value := softWrapApp(t, TextAreaOptions{Value: "aaa bbb ccc ddd"}, 6, 2)
	app.HandleInput(input.KeyEvent{Key: input.KeyHome, Mod: input.ModCtrl})
	app.HandleInput(input.KeyEvent{Key: input.KeyPgDown}) // 2 rows down: "ccc "
	typeRune(app, 'Z')
	if want := "aaa bbb Zccc ddd"; *value != want {
		t.Fatalf("value = %q, want %q", *value, want)
	}
	if rows := screenRows(app); !strings.HasPrefix(rows[0], "Zccc") && !strings.HasPrefix(rows[1], "Zccc") {
		t.Errorf("rows = %q, want the cursor's row scrolled into view", rows)
	}
}

func TestTextAreaSoftWrapClickOnContinuationRow(t *testing.T) {
	app, value := softWrapApp(t, TextAreaOptions{Value: "aaa bbb ccc"}, 6, 4)
	app.HandleInput(input.MouseEvent{X: 1, Y: 1, Button: input.MouseLeft})
	typeRune(app, 'Z')
	if want := "aaa bZbb ccc"; *value != want {
		t.Fatalf("value = %q, want %q", *value, want)
	}
}

func lineNumberGutter(lineIdx int) (string, cell.Style) {
	return strconv.Itoa(lineIdx + 1), style.DefaultDark().MutedText()
}

func TestTextAreaSoftWrapGutterNumbersFirstRowOnly(t *testing.T) {
	app, _ := softWrapApp(t, TextAreaOptions{Value: "aaa bbb ccc\nx", Gutter: lineNumberGutter}, 8, 5)
	got := screenRows(app)[:4]
	want := []string{"1 aaa", "  bbb", "  ccc", "2 x"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("rows = %q, want %q", got, want)
	}
	// The blank continuation gutter keeps the line's own style.
	if got, want := app.Buffer().At(0, 1).Style, style.DefaultDark().MutedText(); got != want {
		t.Errorf("continuation gutter style = %+v, want %+v", got, want)
	}
}

func TestTextAreaSoftWrapClickAccountsForGutter(t *testing.T) {
	app, value := softWrapApp(t, TextAreaOptions{Value: "aaa bbb ccc\nx", Gutter: lineNumberGutter}, 8, 5)
	app.HandleInput(input.MouseEvent{X: 3, Y: 1, Button: input.MouseLeft}) // 'b' at text col 1 of row 1
	typeRune(app, 'Z')
	if want := "aaa bZbb ccc\nx"; *value != want {
		t.Fatalf("value = %q, want %q", *value, want)
	}
}

func TestTextAreaClickAccountsForGutter(t *testing.T) {
	// Unwrapped too: a click's column is relative to the text, not to
	// the gutter's left edge.
	app, value := textAreaApp(t, TextAreaOptions{Theme: style.DefaultDark(), Value: "abcd\nxy", Frameless: true, Gutter: lineNumberGutter})
	app.HandleInput(input.MouseEvent{X: 3, Y: 0, Button: input.MouseLeft}) // gutter is 2 wide: text col 1
	typeRune(app, 'Z')
	if want := "aZbcd\nxy"; *value != want {
		t.Fatalf("value = %q, want %q", *value, want)
	}
}

func TestTextAreaSoftWrapHighlightsAndSelectionSpanRows(t *testing.T) {
	red := cell.Style{Fg: cell.RGBColor(255, 0, 0)}
	app, _ := softWrapApp(t, TextAreaOptions{
		Value:      "aaa bbb ccc",
		Highlights: []StyleSpan{{Start: 2, End: 6, Style: red}}, // "a bb", across the row 0/1 break
	}, 6, 4)
	if got := app.Buffer().At(2, 0).Style.Fg; got != red.Fg {
		t.Errorf("row 0 'a' Fg = %v, want the highlight", got)
	}
	if got := app.Buffer().At(1, 1).Style.Fg; got != red.Fg {
		t.Errorf("row 1 'b' Fg = %v, want the highlight to continue onto the next row", got)
	}

	app.HandleInput(input.KeyEvent{Key: input.KeyHome, Mod: input.ModCtrl})
	app.HandleInput(input.KeyEvent{Key: input.KeyRight, Mod: input.ModShift | input.ModCtrl}) // select "aaa"
	app.HandleInput(input.KeyEvent{Key: input.KeyDown, Mod: input.ModShift})                  // ...through "bbb"
	if app.Buffer().At(1, 1).Style.Attr&cell.AttrReverse == 0 {
		t.Error("selection doesn't continue onto the wrapped row")
	}
}
