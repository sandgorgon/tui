package vt

import (
	"testing"

	"github.com/sandgorgon/tui/cell"
)

// This file is the M4 "headless conformance suite" (docs/DESIGN.md
// §10): hand-written escape sequences approximating what real
// programs — a pager, htop, tmux, a colored shell prompt — actually
// emit, verified end to end rather than one feature in isolation.

func TestConformanceLessLikePager(t *testing.T) {
	s := NewScreen(20, 5)
	p := NewParser()

	// Enter alt screen, clear, print content, then a reverse-video
	// status line at the bottom.
	seq := "\x1b[?1049h\x1b[2J\x1b[H" +
		"line one\r\n" +
		"line two\r\n" +
		"line three\r\n" +
		"line four\r\n" +
		"\x1b[5;1H\x1b[7m(END)\x1b[0m"
	p.Feed([]byte(seq), s)

	want := rows(20, "line one", "line two", "line three", "line four", "(END)")
	if got := dump(s); got != want {
		t.Errorf("dump = %q, want %q", got, want)
	}
	if c := s.Buffer().At(0, 4); c.Style.Attr&cell.AttrReverse == 0 {
		t.Error("status line should be reverse-video")
	}

	// Quit: leave alt screen, back to whatever was on the primary
	// screen (blank, since nothing was ever printed there).
	p.Feed([]byte("\x1b[?1049l"), s)
	if s.useAlt {
		t.Error("should be back on primary screen after quitting")
	}
	if got, want := dump(s), rows(20, "", "", "", "", ""); got != want {
		t.Errorf("primary screen after quit = %q, want blank %q", got, want)
	}
}

func TestConformanceHtopLikePeriodicRedraw(t *testing.T) {
	s := NewScreen(30, 6)
	p := NewParser()

	seq := "\x1b[?1049h\x1b[2J" +
		"\x1b[1;1H\x1b[7m CPU  MEM  TASKS \x1b[0m" +
		"\x1b[2;1H\x1b[32mproc1\x1b[0m running" +
		"\x1b[3;1H\x1b[32mproc2\x1b[0m running"
	p.Feed([]byte(seq), s)

	if c := s.Buffer().At(0, 0); c.Style.Attr&cell.AttrReverse == 0 {
		t.Error("header should be reverse-video")
	}
	if c := s.Buffer().At(0, 1); c.Style.Fg != cell.ANSIColor(2) {
		t.Errorf("proc1 fg = %+v, want green (ANSI 2)", c.Style.Fg)
	}

	// A periodic redraw: move to the row, clear it, print updated info
	// — exactly how htop/top refresh a single line without touching
	// the rest of the screen.
	p.Feed([]byte("\x1b[2;1H\x1b[2K\x1b[32mproc1\x1b[0m stopped"), s)

	want := rows(30, " CPU  MEM  TASKS ", "proc1 stopped", "proc2 running", "", "", "")
	if got := dump(s); got != want {
		t.Errorf("dump after redraw = %q, want %q", got, want)
	}
}

func TestConformanceTmuxLikeStatusLineSurvivesScroll(t *testing.T) {
	s := NewScreen(10, 4)
	p := NewParser()

	// Confine scrolling to rows 1-3 (1-based), reserving row 4 as a
	// fixed status line — exactly how tmux/screen keep their status
	// bar in place while the pane above it scrolls.
	p.Feed([]byte("\x1b[1;3r\x1b[4;1H\x1b[7m status \x1b[0m\x1b[1;1H"), s)
	p.Feed([]byte("one\r\ntwo\r\nthree\r\nfour"), s)

	want := rows(10, "two", "three", "four", " status ")
	if got := dump(s); got != want {
		t.Errorf("dump = %q, want %q", got, want)
	}
	if c := s.Buffer().At(0, 3); c.Style.Attr&cell.AttrReverse == 0 {
		t.Error("status line should still be reverse-video after scrolling")
	}
}

func TestConformanceShellPromptTruecolor(t *testing.T) {
	s := NewScreen(40, 1)
	p := NewParser()

	// A starship/powerline-style prompt segment: truecolor background
	// block plus foreground text, colon-extended SGR form.
	seq := "\x1b[48:2:30:30:30m\x1b[38:2:0:255:0m user@host \x1b[0m "
	p.Feed([]byte(seq), s)

	c := s.Buffer().At(0, 0)
	if want := cell.RGBColor(30, 30, 30); c.Style.Bg != want {
		t.Errorf("Bg = %+v, want %+v", c.Style.Bg, want)
	}
	if want := cell.RGBColor(0, 255, 0); c.Style.Fg != want {
		t.Errorf("Fg = %+v, want %+v", c.Style.Fg, want)
	}
}

func TestConformanceVimLikeSessionWithLineEditing(t *testing.T) {
	s := NewScreen(20, 4)
	p := NewParser()

	// vim-like startup: alt screen, box-drawing borders via DEC
	// special graphics, a status line, then a query the terminal must
	// answer for vim to detect truecolor/cursor-position support.
	seq := "\x1b[?1049h\x1b[2J" +
		"\x1b(0lqqqqqqqqqqqqqqqqqqk\x1b(B\r\n" + // ┌───...───┐ top border
		"~\r\n" +
		"\x1b[3;1H-- INSERT --" +
		"\x1b[6n" // ask for cursor position (CPR)
	p.Feed([]byte(seq), s)

	top := s.Buffer().At(0, 0)
	if top.Rune != '┌' {
		t.Errorf("top-left border cell = %q, want '┌'", top.Rune)
	}
	mid := s.Buffer().At(1, 0)
	if mid.Rune != '─' {
		t.Errorf("border fill cell = %q, want '─'", mid.Rune)
	}
	last := s.Buffer().At(19, 0)
	if last.Rune != '┐' {
		t.Errorf("top-right border cell = %q, want '┐'", last.Rune)
	}

	statusWant := rows(20, "┌──────────────────┐", "~", "-- INSERT --", "")
	if got := dump(s); got != statusWant {
		t.Errorf("dump = %q, want %q", got, statusWant)
	}

	// vim needs a real CPR reply (cursor is after "-- INSERT --", i.e.
	// column 13 on row 3, 1-based) to know the terminal is responsive.
	if got, want := string(s.TakeResponses()), "\x1b[3;13R"; got != want {
		t.Errorf("CPR response = %q, want %q", got, want)
	}

	// Quit: leave alt screen.
	p.Feed([]byte("\x1b[?1049l"), s)
	if s.useAlt {
		t.Error("should be back on primary screen after quitting vim")
	}
}
