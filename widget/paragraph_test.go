package widget

import (
	"reflect"
	"testing"

	"github.com/sandgorgon/tui/cell"
)

func TestWrapTextBasic(t *testing.T) {
	got := wrapText("the quick brown fox", 10)
	want := []string{"the quick", "brown fox"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("wrapText = %v, want %v", got, want)
	}
}

func TestWrapTextHardBreaksOverlongWord(t *testing.T) {
	got := wrapText("hello world foobarbazquux", 5)
	want := []string{"hello", "world", "fooba", "rbazq", "uux"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("wrapText = %v, want %v", got, want)
	}
}

func TestWrapTextPreservesHardBreaks(t *testing.T) {
	got := wrapText("a b\n\nc d", 10)
	want := []string{"a b", "", "c d"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("wrapText = %v, want %v", got, want)
	}
}

func TestWrapTextEmpty(t *testing.T) {
	if got := wrapText("", 10); !reflect.DeepEqual(got, []string{""}) {
		t.Errorf("wrapText(\"\") = %v, want [\"\"]", got)
	}
}

func TestWrapTextZeroWidth(t *testing.T) {
	if got := wrapText("hi", 0); got != nil {
		t.Errorf("wrapText with width 0 = %v, want nil", got)
	}
}

// TestStringWidthAgreesWithPainterTextOnZeroWidthRunes guards against a
// real bug: wcwidth.RuneWidth returns 0 for a combining mark (genuinely
// zero-width in a terminal that composes it onto the preceding base
// character), but cell.Painter doesn't do combining-mark composition
// (see Painter's own doc comment) — it draws every rune as its own
// cell, so it treats a 0-width rune as occupying 1 column, same as any
// other unhandled non-positive width. stringWidth/runeWidth used to
// only special-case negative widths (control chars), leaving 0 as 0 —
// silently disagreeing with what Painter.Text actually draws, which
// under-reserved space for any caller measuring text before painting it
// (e.g. widget.StatusBar's center/right segment layout).
func TestStringWidthAgreesWithPainterTextOnZeroWidthRunes(t *testing.T) {
	const combiningAcute = "é" // "e" + COMBINING ACUTE ACCENT (zero-width)

	buf := cell.NewBuffer(10, 1)
	painted := cell.NewPainter(buf).Text(0, 0, combiningAcute, cell.Style{})

	if measured := stringWidth(combiningAcute); measured != painted {
		t.Errorf("stringWidth(%q) = %d, but Painter.Text painted %d columns — they must agree", combiningAcute, measured, painted)
	}
}

func TestParagraphPaint(t *testing.T) {
	node := Paragraph("the quick brown fox jumps", cell.Style{})
	buf := cell.NewBuffer(11, 3)
	paintNode(t, node, buf)

	want := "the quick  \nbrown fox  \njumps      "
	if got := buf.String(); got != want {
		t.Errorf("Buffer =\n%q\nwant\n%q", got, want)
	}
}

func TestParagraphPaintClipsExcessLines(t *testing.T) {
	node := Paragraph("one two three four five", cell.Style{})
	buf := cell.NewBuffer(4, 2) // only 2 rows: "one\n", "two\n" fit, the rest clipped
	paintNode(t, node, buf)

	want := "one \ntwo "
	if got := buf.String(); got != want {
		t.Errorf("Buffer = %q, want %q", got, want)
	}
}
