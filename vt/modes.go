package vt

import "github.com/sandgorgon/tui/cell"

// MouseMode identifies which xterm mouse-reporting mode (if any) an
// application has requested via DECSET, so a host knows what kind of
// mouse events (if any) to translate and forward to the child — see
// docs/DESIGN.md §7's transparent-passthrough note.
type MouseMode uint8

const (
	MouseOff         MouseMode = iota
	MouseX10                   // 9: press only
	MouseNormal                // 1000: press + release
	MouseHighlight             // 1001: rarely used in practice, treated like Normal
	MouseButtonEvent           // 1002: press + release + drag while a button is held
	MouseAnyEvent              // 1003: press + release + all motion
)

// MouseEncoding identifies how mouse reports are encoded on the wire.
type MouseEncoding uint8

const (
	MouseEncodingDefault MouseEncoding = iota // legacy X10 encoding
	MouseEncodingUTF8                         // 1005
	MouseEncodingSGR                          // 1006
	MouseEncodingURXVT                        // 1015
)

// setMode implements SM/RM (private == 0, ANSI modes) and DECSET/
// DECRST (private == '?', DEC private modes).
func (s *Screen) setMode(private byte, params CSIParams, set bool) {
	for _, p := range params.Ints() {
		if private == '?' {
			s.setDECMode(p, set)
		} else {
			s.setANSIMode(p, set)
		}
	}
}

func (s *Screen) setANSIMode(p int, set bool) {
	switch p {
	case 4: // IRM insert mode
		s.insertMode = set
	}
}

func (s *Screen) setDECMode(p int, set bool) {
	switch p {
	case 1: // DECCKM: application cursor keys — a host's key encoder cares about this
		s.appCursorKeys = set
	case 6: // DECOM origin mode; spec: changing it homes the cursor
		s.originMode = set
		s.moveTo(1, 1)
	case 7: // DECAWM autowrap
		s.autowrap = set
	case 9:
		s.mouseMode = ternary(set, MouseX10, MouseOff)
	case 25: // DECTCEM cursor visibility
		s.cursorVisible = set
	case 47:
		s.setAltScreen(set, false)
	case 1000:
		s.mouseMode = ternary(set, MouseNormal, MouseOff)
	case 1001:
		s.mouseMode = ternary(set, MouseHighlight, MouseOff)
	case 1002:
		s.mouseMode = ternary(set, MouseButtonEvent, MouseOff)
	case 1003:
		s.mouseMode = ternary(set, MouseAnyEvent, MouseOff)
	case 1004:
		s.focusEvents = set
	case 1005:
		s.mouseEncoding = ternary(set, MouseEncodingUTF8, MouseEncodingDefault)
	case 1006:
		s.mouseEncoding = ternary(set, MouseEncodingSGR, MouseEncodingDefault)
	case 1015:
		s.mouseEncoding = ternary(set, MouseEncodingURXVT, MouseEncodingDefault)
	case 1047:
		s.setAltScreen(set, false)
	case 1049:
		s.setAltScreen(set, true)
	case 2004:
		s.bracketedPaste = set
	case 2026:
		s.synchronizedOutput = set
	}
}

func ternary[T any](cond bool, t, f T) T {
	if cond {
		return t
	}
	return f
}

// setAltScreen implements DEC modes 47/1047 (switch screens, clearing
// the alternate screen on entry) and 1049 (the same, plus an implicit
// cursor save/restore — what modern full-screen apps like vim/less/
// htop/tmux actually use almost exclusively).
func (s *Screen) setAltScreen(enter, saveCursor bool) {
	if enter == s.useAlt {
		return
	}
	if enter {
		if saveCursor {
			s.saveCursorState()
		}
		s.useAlt = true
		s.clearActiveScreen()
	} else {
		s.useAlt = false
		if saveCursor {
			s.restoreCursorState()
		}
	}
}

func (s *Screen) clearActiveScreen() {
	buf := s.activeBuffer()
	for y := 0; y < s.rows; y++ {
		s.clearRow(buf, y)
	}
}

// saveCursorState implements DECSC (ESC 7) and mode 1049's implicit
// save. The primary and alternate screens keep independent saved-
// cursor slots, matching real terminals.
func (s *Screen) saveCursorState() {
	st := savedCursorState{
		x: s.cx, y: s.cy, style: s.curStyle,
		autowrap: s.autowrap, originMode: s.originMode,
		g0: s.g0, g1: s.g1, glActive: s.glActive,
		wrapPending: s.wrapPending,
	}
	if s.useAlt {
		s.savedAlt = st
	} else {
		s.saved = st
	}
}

// restoreCursorState implements DECRC (ESC 8) and mode 1049's implicit
// restore.
func (s *Screen) restoreCursorState() {
	st := s.saved
	if s.useAlt {
		st = s.savedAlt
	}
	s.cx, s.cy = st.x, st.y
	s.curStyle = st.style
	s.autowrap = st.autowrap
	s.originMode = st.originMode
	s.g0, s.g1 = st.g0, st.g1
	s.glActive = st.glActive
	s.wrapPending = st.wrapPending
	s.clampCursor()
}

// setScrollRegion implements DECSTBM (CSI r): top/bottom are 1-based
// and inclusive, as CSI sends them; an invalid or omitted region
// resets to the full screen. Spec: it also homes the cursor.
func (s *Screen) setScrollRegion(top, bottom int) {
	if top < 1 {
		top = 1
	}
	if bottom < 1 || bottom > s.rows {
		bottom = s.rows
	}
	if top >= bottom {
		top, bottom = 1, s.rows
	}
	s.scrollTop = top - 1
	s.scrollBottom = bottom - 1
	s.moveTo(1, 1)
}

// alignmentTest implements DECALN (ESC # 8): fills the screen with 'E'
// and resets margins, used by some programs/terminfo test sequences to
// visually verify cell alignment.
func (s *Screen) alignmentTest() {
	buf := s.activeBuffer()
	e := cell.Cell{Rune: 'E', Width: 1}
	for y := 0; y < s.rows; y++ {
		for x := 0; x < s.cols; x++ {
			buf.Set(x, y, e)
		}
	}
	s.cx, s.cy = 0, 0
	s.scrollTop, s.scrollBottom = 0, s.rows-1
}
