package vt

import (
	"strings"
	"testing"

	"github.com/sandgorgon/tui/cell"
)

func newTestScreen(cols, rows int, seqs ...string) *Screen {
	s := NewScreen(cols, rows)
	p := NewParser()
	for _, seq := range seqs {
		p.Feed([]byte(seq), s)
	}
	return s
}

func feedStr(s *Screen, seq string) {
	NewParser().Feed([]byte(seq), s)
}

func dump(s *Screen) string {
	return s.Buffer().String()
}

func padRow(s string, cols int) string {
	for len([]rune(s)) < cols {
		s += " "
	}
	return s
}

func rows(cols int, lines ...string) string {
	padded := make([]string, len(lines))
	for i, l := range lines {
		padded[i] = padRow(l, cols)
	}
	return strings.Join(padded, "\n")
}

func cellsToString(cs []cell.Cell) string {
	var sb strings.Builder
	for _, c := range cs {
		if c.IsContinuation() {
			continue
		}
		r := c.Rune
		if r == 0 {
			r = ' '
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

func TestPrintAdvancesCursor(t *testing.T) {
	s := newTestScreen(5, 2, "abc")
	x, y, visible := s.Cursor()
	if x != 3 || y != 0 || !visible {
		t.Errorf("cursor = (%d,%d,visible=%v), want (3,0,true)", x, y, visible)
	}
	if got, want := dump(s), rows(5, "abc", ""); got != want {
		t.Errorf("dump = %q, want %q", got, want)
	}
}

func TestCRLF(t *testing.T) {
	s := newTestScreen(5, 2, "ab\r\ncd")
	if got, want := dump(s), rows(5, "ab", "cd"); got != want {
		t.Errorf("dump = %q, want %q", got, want)
	}
	x, y, _ := s.Cursor()
	if x != 2 || y != 1 {
		t.Errorf("cursor = (%d,%d), want (2,1)", x, y)
	}
}

func TestDeferredAutowrapNoSpuriousBlankLine(t *testing.T) {
	// A line that exactly fills the width, then a newline: without
	// deferred-wrap semantics, this produces a spurious extra blank
	// line between "abcde" and "next".
	s := newTestScreen(5, 3, "abcde\r\nnext")
	if got, want := dump(s), rows(5, "abcde", "next", ""); got != want {
		t.Errorf("dump = %q, want %q", got, want)
	}
}

func TestAutowrapWrapsOnNextChar(t *testing.T) {
	s := newTestScreen(5, 2, "abcdeZ")
	if got, want := dump(s), rows(5, "abcde", "Z"); got != want {
		t.Errorf("dump = %q, want %q", got, want)
	}
	x, y, _ := s.Cursor()
	if x != 1 || y != 1 {
		t.Errorf("cursor = (%d,%d), want (1,1)", x, y)
	}
}

func TestAutowrapOffPinsCursorAndOverwritesLastColumn(t *testing.T) {
	// With autowrap off, printing past the last column doesn't wrap —
	// the cursor sticks at the last column and keeps overwriting it,
	// matching real terminal behavior (it does not discard overflow).
	s := newTestScreen(5, 2, "\x1b[?7labcdeZ") // DECRST 7: disable autowrap
	if got, want := dump(s), rows(5, "abcdZ", ""); got != want {
		t.Errorf("dump = %q, want %q", got, want)
	}
	x, y, _ := s.Cursor()
	if x != 4 || y != 0 {
		t.Errorf("cursor = (%d,%d), want (4,0) pinned at the last column", x, y)
	}
}

func TestCursorMovementClamping(t *testing.T) {
	s := newTestScreen(5, 5)
	feedStr(s, "\x1b[100B")
	if _, y, _ := s.Cursor(); y != 4 {
		t.Errorf("y = %d, want 4 (clamped)", y)
	}
	feedStr(s, "\x1b[100C")
	if x, _, _ := s.Cursor(); x != 4 {
		t.Errorf("x = %d, want 4 (clamped)", x)
	}
}

func TestCUP(t *testing.T) {
	s := newTestScreen(5, 5, "\x1b[3;2Hx")
	x, y, _ := s.Cursor()
	if x != 2 || y != 2 {
		t.Errorf("cursor = (%d,%d), want (2,2)", x, y)
	}
	if got, want := dump(s), rows(5, "", "", " x", "", ""); got != want {
		t.Errorf("dump = %q, want %q", got, want)
	}
}

func TestEraseInDisplayCursorToEnd(t *testing.T) {
	s := newTestScreen(3, 3, "abc\r\ndef\r\nghi\x1b[2;2H\x1b[0J")
	if got, want := dump(s), rows(3, "abc", "d", ""); got != want {
		t.Errorf("dump = %q, want %q", got, want)
	}
}

func TestEraseInDisplayWholeScreen(t *testing.T) {
	s := newTestScreen(3, 2, "abc\r\ndef\x1b[2J")
	if got, want := dump(s), rows(3, "", ""); got != want {
		t.Errorf("dump = %q, want %q", got, want)
	}
}

func TestScrollRegionConfinesScrolling(t *testing.T) {
	s := newTestScreen(3, 5, "111\r\n222\r\n333\r\n444\r\n555")
	feedStr(s, "\x1b[2;4r\x1b[4;1H") // region rows 2-4 (1-based); cursor to its bottom
	feedStr(s, "\n666")

	want := rows(3, "111", "333", "444", "666", "555")
	if got := dump(s); got != want {
		t.Errorf("dump = %q, want %q", got, want)
	}
}

func TestFullScreenScrollPushesToScrollback(t *testing.T) {
	s := newTestScreen(3, 2, "aaa\r\nbbb\r\nccc")
	if s.ScrollbackLen() != 1 {
		t.Fatalf("ScrollbackLen() = %d, want 1", s.ScrollbackLen())
	}
	if got := cellsToString(s.ScrollbackLine(0)); got != "aaa" {
		t.Errorf("scrollback line = %q, want %q", got, "aaa")
	}
	if got, want := dump(s), rows(3, "bbb", "ccc"); got != want {
		t.Errorf("dump = %q, want %q", got, want)
	}
}

func TestScrollRegionScrollDoesNotPushScrollback(t *testing.T) {
	s := newTestScreen(3, 4, "aaa\r\nbbb\r\nccc\r\nddd")
	feedStr(s, "\x1b[2;4r\x1b[4;1H\n")
	if s.ScrollbackLen() != 0 {
		t.Errorf("ScrollbackLen() = %d, want 0 (region doesn't start at row 0)", s.ScrollbackLen())
	}
}

func TestAltScreenPreservesPrimaryAndCursor(t *testing.T) {
	s := newTestScreen(5, 3, "hello\x1b[2;3H")
	primaryDump := dump(s)
	px, py, _ := s.Cursor()

	feedStr(s, "\x1b[?1049h")
	if dump(s) == primaryDump {
		t.Error("alt screen should start cleared, not equal to primary content")
	}
	feedStr(s, "ALT")
	feedStr(s, "\x1b[?1049l")

	if got := dump(s); got != primaryDump {
		t.Errorf("primary screen after returning from alt = %q, want unchanged %q", got, primaryDump)
	}
	x, y, _ := s.Cursor()
	if x != px || y != py {
		t.Errorf("cursor after returning from alt = (%d,%d), want restored (%d,%d)", x, y, px, py)
	}
}

func TestSGRBasicAttributes(t *testing.T) {
	s := newTestScreen(5, 1, "\x1b[1;4mX")
	c := s.Buffer().At(0, 0)
	if c.Style.Attr&cell.AttrBold == 0 {
		t.Error("expected AttrBold set")
	}
	if c.Style.Underline != cell.UnderlineSingle {
		t.Errorf("Underline = %v, want UnderlineSingle", c.Style.Underline)
	}
}

func TestSGRTruecolorLegacyForm(t *testing.T) {
	s := newTestScreen(5, 1, "\x1b[38;2;10;20;30mX")
	c := s.Buffer().At(0, 0)
	if want := cell.RGBColor(10, 20, 30); c.Style.Fg != want {
		t.Errorf("Fg = %+v, want %+v", c.Style.Fg, want)
	}
}

func TestSGRTruecolorColonForm(t *testing.T) {
	s := newTestScreen(5, 1, "\x1b[38:2:10:20:30mX")
	c := s.Buffer().At(0, 0)
	if want := cell.RGBColor(10, 20, 30); c.Style.Fg != want {
		t.Errorf("Fg = %+v, want %+v", c.Style.Fg, want)
	}
}

func TestSGRTruecolorColonFormWithEmptyColorspace(t *testing.T) {
	s := newTestScreen(5, 1, "\x1b[38:2::10:20:30mX")
	c := s.Buffer().At(0, 0)
	if want := cell.RGBColor(10, 20, 30); c.Style.Fg != want {
		t.Errorf("Fg = %+v, want %+v", c.Style.Fg, want)
	}
}

func TestSGRIndexedColorBothForms(t *testing.T) {
	want := cell.IndexedColor(200)
	if c := newTestScreen(5, 1, "\x1b[38;5;200mX").Buffer().At(0, 0); c.Style.Fg != want {
		t.Errorf("legacy form Fg = %+v, want %+v", c.Style.Fg, want)
	}
	if c := newTestScreen(5, 1, "\x1b[38:5:200mX").Buffer().At(0, 0); c.Style.Fg != want {
		t.Errorf("colon form Fg = %+v, want %+v", c.Style.Fg, want)
	}
}

func TestSGRResetClearsAttributes(t *testing.T) {
	s := newTestScreen(5, 1, "\x1b[1;31mA\x1b[0mB")
	a := s.Buffer().At(0, 0)
	b := s.Buffer().At(1, 0)
	if a.Style.Attr&cell.AttrBold == 0 {
		t.Error("A should be bold")
	}
	if b.Style != (cell.Style{}) {
		t.Errorf("B style after reset = %+v, want zero Style", b.Style)
	}
}

func TestDECSCDECRC(t *testing.T) {
	s := newTestScreen(5, 5, "\x1b[3;3H\x1b[1m\x1b7\x1b[1;1H\x1b[0mX\x1b8Y")
	xc := s.Buffer().At(0, 0)
	if xc.Rune != 'X' || xc.Style.Attr&cell.AttrBold != 0 {
		t.Errorf("X cell = %+v, want plain 'X'", xc)
	}
	yc := s.Buffer().At(2, 2)
	if yc.Rune != 'Y' || yc.Style.Attr&cell.AttrBold == 0 {
		t.Errorf("Y cell = %+v, want bold 'Y' at (2,2)", yc)
	}
}

func TestOriginModeConfinesCUP(t *testing.T) {
	s := newTestScreen(5, 5, "\x1b[2;4r\x1b[?6h\x1b[1;1H")
	if _, y, _ := s.Cursor(); y != 1 {
		t.Errorf("y = %d, want 1 (region top under origin mode)", y)
	}
}

func TestOSCTitle(t *testing.T) {
	s := newTestScreen(5, 1, "\x1b]2;My Title\x07")
	if s.Title() != "My Title" {
		t.Errorf("Title() = %q, want %q", s.Title(), "My Title")
	}
}

func TestOSCHyperlink(t *testing.T) {
	s := newTestScreen(20, 1, "\x1b]8;;https://example.com\x07link\x1b]8;;\x07plain")
	c := s.Buffer().At(0, 0)
	if c.Style.Hyperlink != "https://example.com" {
		t.Errorf("hyperlinked cell Hyperlink = %q, want the URI", c.Style.Hyperlink)
	}
	c2 := s.Buffer().At(4, 0) // 'p' of "plain", after the link closed
	if c2.Style.Hyperlink != "" {
		t.Errorf("cell after closing hyperlink has Hyperlink = %q, want empty", c2.Style.Hyperlink)
	}
}

func TestDA1Response(t *testing.T) {
	s := newTestScreen(5, 1, "\x1b[c")
	if got, want := string(s.TakeResponses()), "\x1b[?1;2c"; got != want {
		t.Errorf("DA1 response = %q, want %q", got, want)
	}
	if len(s.TakeResponses()) != 0 {
		t.Error("TakeResponses should be empty after being taken")
	}
}

func TestCPRResponse(t *testing.T) {
	s := newTestScreen(10, 10, "\x1b[4;6H\x1b[6n")
	if got, want := string(s.TakeResponses()), "\x1b[4;6R"; got != want {
		t.Errorf("CPR response = %q, want %q", got, want)
	}
}

func TestTabStops(t *testing.T) {
	s := newTestScreen(20, 1, "\tX")
	x, _, _ := s.Cursor()
	if x != 9 {
		t.Errorf("x = %d, want 9", x)
	}
	if c := s.Buffer().At(8, 0); c.Rune != 'X' {
		t.Errorf("expected X at column 8, got %+v", c)
	}
}

func TestBackTab(t *testing.T) {
	s := newTestScreen(20, 1, "\x1b[20G\x1b[Z")
	if x, _, _ := s.Cursor(); x != 16 {
		t.Errorf("x = %d, want 16 (previous tab stop)", x)
	}
}

func TestDECSpecialGraphics(t *testing.T) {
	s := newTestScreen(5, 1, "\x1b(0q\x1b(Br")
	c := s.Buffer().At(0, 0)
	if c.Rune != '─' {
		t.Errorf("Rune = %q, want '─' (DEC special graphics for 'q')", c.Rune)
	}
	c2 := s.Buffer().At(1, 0)
	if c2.Rune != 'r' {
		t.Errorf("Rune = %q, want plain 'r' after switching back to ASCII", c2.Rune)
	}
}

func TestRISFullReset(t *testing.T) {
	s := newTestScreen(5, 2, "\x1b[1mhello\x1b[?1049h\x1b[?7l\x1bc")
	if !s.autowrap {
		t.Error("autowrap should be reset to true")
	}
	if got, want := dump(s), rows(5, "", ""); got != want {
		t.Errorf("dump = %q, want %q (screen should be cleared after RIS)", got, want)
	}
	if s.useAlt {
		t.Error("should be back on primary screen after RIS")
	}
}

func TestInsertMode(t *testing.T) {
	s := newTestScreen(5, 1, "abc\x1b[1G\x1b[4hX")
	if got, want := dump(s), rows(5, "Xabc"); got != want {
		t.Errorf("dump = %q, want %q", got, want)
	}
}

func TestICH(t *testing.T) {
	s := newTestScreen(5, 1, "abc\x1b[1G\x1b[2@")
	if got, want := dump(s), rows(5, "  abc"); got != want {
		t.Errorf("dump = %q, want %q", got, want)
	}
}

func TestDCH(t *testing.T) {
	s := newTestScreen(5, 1, "abcde\x1b[1G\x1b[2P")
	if got, want := dump(s), rows(5, "cde"); got != want {
		t.Errorf("dump = %q, want %q", got, want)
	}
}

func TestECH(t *testing.T) {
	s := newTestScreen(5, 1, "abcde\x1b[2G\x1b[2X")
	if got, want := dump(s), rows(5, "a  de"); got != want {
		t.Errorf("dump = %q, want %q", got, want)
	}
}

func TestWideRuneOccupiesTwoCells(t *testing.T) {
	s := newTestScreen(5, 1, "a中b")
	c1 := s.Buffer().At(1, 0)
	if c1.Rune != '中' || c1.Width != 2 {
		t.Errorf("cell(1,0) = %+v, want wide '中'", c1)
	}
	c2 := s.Buffer().At(2, 0)
	if !c2.IsContinuation() {
		t.Errorf("cell(2,0) = %+v, want a continuation cell", c2)
	}
	c3 := s.Buffer().At(3, 0)
	if c3.Rune != 'b' {
		t.Errorf("cell(3,0) = %+v, want 'b'", c3)
	}
}

func TestResizePreservesTopLeftContent(t *testing.T) {
	s := newTestScreen(5, 3, "abcde\r\nfghij\r\nklmno")
	s.Resize(3, 2) // shrink: only the top-left 3x2 region should survive
	if got, want := dump(s), rows(3, "abc", "fgh"); got != want {
		t.Errorf("dump after shrink = %q, want %q", got, want)
	}
	if cols, rows := s.Size(); cols != 3 || rows != 2 {
		t.Errorf("Size() = (%d,%d), want (3,2)", cols, rows)
	}
}

func TestResizeGrowClampsScrollRegionAndCursor(t *testing.T) {
	s := newTestScreen(5, 5, "\x1b[2;4r\x1b[5;5H") // scroll region rows 2-4; cursor at bottom-right
	s.Resize(3, 3)                                 // now smaller than the old cursor position and scroll region
	if x, y, _ := s.Cursor(); x != 2 || y != 2 {
		t.Errorf("cursor after shrink = (%d,%d), want clamped to (2,2)", x, y)
	}
	if s.scrollBottom != 2 {
		t.Errorf("scrollBottom after shrink = %d, want clamped to 2", s.scrollBottom)
	}
}

func TestResizeNoOpWhenSameSize(t *testing.T) {
	s := newTestScreen(4, 2, "abcd\r\nefgh")
	before := dump(s)
	s.Resize(4, 2)
	if got := dump(s); got != before {
		t.Errorf("dump after no-op resize = %q, want unchanged %q", got, before)
	}
}
