package vt

// This file implements Parser's Handler interface on Screen, dispatching
// each parsed action to the semantic primitives in screen.go, sgr.go,
// modes.go, osc.go, and response.go.

// Print implements Handler.
func (s *Screen) Print(r rune) { s.print(r) }

// Execute implements Handler: C0 control characters.
func (s *Screen) Execute(b byte) {
	switch b {
	case 0x08: // BS
		if s.cx > 0 {
			s.cx--
		}
		s.wrapPending = false
	case 0x09: // HT
		s.tab()
	case 0x0A, 0x0B, 0x0C: // LF, VT, FF: all treated as index (no CR)
		s.index()
	case 0x0D: // CR
		s.carriageReturn()
	case 0x0E: // SO: invoke G1 into GL
		s.glActive = 1
	case 0x0F: // SI: invoke G0 into GL
		s.glActive = 0
	}
	// BEL (0x07) and other C0 controls not listed have no screen-state
	// effect at this layer; a bell notification is left to a future
	// consumer that wants one (out of scope for the pure screen model).
}

// CSI implements Handler.
func (s *Screen) CSI(private byte, params CSIParams, intermediates []byte, final byte) {
	switch {
	case len(intermediates) == 1 && intermediates[0] == ' ' && final == 'q':
		s.setCursorShape(params.Get(0, 1)) // DECSCUSR
	case private == 0 && len(intermediates) == 0:
		s.csiPlain(params, final)
	case private == '?' && len(intermediates) == 0:
		s.csiPrivate(params, final)
	case private == '>' && len(intermediates) == 0 && final == 'c':
		s.da2()
	}
}

func (s *Screen) csiPlain(params CSIParams, final byte) {
	switch final {
	case 'A': // CUU
		s.moveBy(0, -max(1, params.Get(0, 1)))
	case 'B': // CUD
		s.moveBy(0, max(1, params.Get(0, 1)))
	case 'C': // CUF
		s.moveBy(max(1, params.Get(0, 1)), 0)
	case 'D': // CUB
		s.moveBy(-max(1, params.Get(0, 1)), 0)
	case 'E': // CNL: down N, column 0
		s.moveBy(0, max(1, params.Get(0, 1)))
		s.cx = 0
	case 'F': // CPL: up N, column 0
		s.moveBy(0, -max(1, params.Get(0, 1)))
		s.cx = 0
	case 'G': // CHA
		s.moveToColumn(params.Get(0, 1))
	case 'H', 'f': // CUP / HVP
		s.moveTo(params.Get(0, 1), params.Get(1, 1))
	case 'I': // CHT: forward tabs
		for range max(1, params.Get(0, 1)) {
			s.tab()
		}
	case 'J': // ED
		s.eraseInDisplay(params.Get(0, 0))
	case 'K': // EL
		s.eraseInLine(params.Get(0, 0))
	case 'L': // IL
		s.scrollRegionDown(s.cy, s.scrollBottom, max(1, params.Get(0, 1)))
	case 'M': // DL
		s.scrollRegionUp(s.cy, s.scrollBottom, max(1, params.Get(0, 1)), false)
	case 'P': // DCH
		s.deleteCells(max(1, params.Get(0, 1)))
	case 'S': // SU
		s.scrollRegionUp(s.scrollTop, s.scrollBottom, max(1, params.Get(0, 1)), true)
	case 'T': // SD
		s.scrollRegionDown(s.scrollTop, s.scrollBottom, max(1, params.Get(0, 1)))
	case 'X': // ECH
		s.eraseCellsAtCursor(max(1, params.Get(0, 1)))
	case 'Z': // CBT: back tabs
		for range max(1, params.Get(0, 1)) {
			s.backTab()
		}
	case '@': // ICH
		s.insertBlanks(max(1, params.Get(0, 1)))
	case 'c': // DA1
		s.da1()
	case 'd': // VPA
		s.moveToRow(params.Get(0, 1))
	case 'g': // TBC
		s.clearTabStops(params.Get(0, 0))
	case 'h': // SM
		s.setMode(0, params, true)
	case 'l': // RM
		s.setMode(0, params, false)
	case 'm': // SGR
		s.sgr(params)
	case 'n': // DSR
		s.dsr(params)
	case 'r': // DECSTBM
		s.setScrollRegion(params.Get(0, 1), params.Get(1, s.rows))
	}
}

func (s *Screen) csiPrivate(params CSIParams, final byte) {
	switch final {
	case 'h': // DECSET
		s.setMode('?', params, true)
	case 'l': // DECRST
		s.setMode('?', params, false)
	}
}

// ESC implements Handler.
func (s *Screen) ESC(intermediates []byte, final byte) {
	if len(intermediates) == 0 {
		switch final {
		case '7': // DECSC
			s.saveCursorState()
		case '8': // DECRC
			s.restoreCursorState()
		case 'D': // IND
			s.index()
		case 'E': // NEL
			s.newline()
		case 'H': // HTS
			if s.cx < len(s.tabStops) {
				s.tabStops[s.cx] = true
			}
		case 'M': // RI
			s.reverseIndex()
		case 'c': // RIS
			s.reset()
		}
		return
	}
	if len(intermediates) == 1 {
		switch intermediates[0] {
		case '(': // designate G0
			s.g0 = charsetFromFinal(final)
		case ')': // designate G1
			s.g1 = charsetFromFinal(final)
		case '#':
			if final == '8' {
				s.alignmentTest() // DECALN
			}
		}
	}
}

func charsetFromFinal(final byte) charset {
	if final == '0' {
		return charsetDECSpecialGraphics
	}
	return charsetASCII
}

// OSC implements Handler.
func (s *Screen) OSC(data []byte) { s.osc(data) }
