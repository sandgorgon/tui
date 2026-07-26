package style

import (
	"testing"

	"github.com/sandgorgon/tui/cell"
)

func TestBuilderComposesStyle(t *testing.T) {
	got := New(cell.ANSIColor(4)).
		Bg(cell.ANSIColor(0)).
		Bold().
		Underline(cell.UnderlineCurly).
		Style()

	want := cell.Style{
		Fg:        cell.ANSIColor(4),
		Bg:        cell.ANSIColor(0),
		Attr:      cell.AttrBold,
		Underline: cell.UnderlineCurly,
	}
	if got != want {
		t.Errorf("Style() = %+v, want %+v", got, want)
	}
}

func TestBuilderMethodsReturnIndependentCopies(t *testing.T) {
	base := New(cell.ANSIColor(1))
	bold := base.Bold()
	italic := base.Italic()

	if bold.Style().Attr&cell.AttrItalic != 0 {
		t.Error("deriving italic from base shouldn't have mutated bold")
	}
	if italic.Style().Attr&cell.AttrBold != 0 {
		t.Error("deriving bold from base shouldn't have mutated italic")
	}
	if base.Style().Attr != 0 {
		t.Error("base itself should be unmodified by either derivation")
	}
}

func TestFromStartsWithExistingStyle(t *testing.T) {
	base := cell.Style{Fg: cell.ANSIColor(2), Attr: cell.AttrItalic}
	got := From(base).Bold().Style()

	want := cell.Style{Fg: cell.ANSIColor(2), Attr: cell.AttrItalic | cell.AttrBold}
	if got != want {
		t.Errorf("Style() = %+v, want %+v", got, want)
	}
}
