package widget

import (
	"fmt"
	"unicode"

	"github.com/sandgorgon/tui/input"
)

// encodeEvent is the reverse of package input's Decoder: it turns a
// decoded input.Event back into the byte sequence a real terminal
// would have sent for it, for Terminal to write to its pty's master —
// package input only ever decoded bytes before this (see
// docs/DESIGN.md §4), since nothing needed the reverse direction until
// a widget had to forward keystrokes into a child process. Each
// sequence here is deliberately the exact inverse of what
// input/decode.go parses for it, so the two stay consistent by
// construction rather than by two independently-written encodings
// happening to agree.
func encodeEvent(e input.Event) []byte {
	switch ev := e.(type) {
	case input.KeyEvent:
		return encodeKey(ev)
	case input.MouseEvent:
		return encodeMouse(ev)
	case input.PasteEvent:
		return append(append([]byte("\x1b[200~"), []byte(ev.Text)...), "\x1b[201~"...)
	default:
		return nil
	}
}

func encodeKey(ke input.KeyEvent) []byte {
	if ke.Key == input.KeyNone && ke.Rune != 0 {
		if ke.Mod&input.ModCtrl != 0 {
			r := unicode.ToUpper(ke.Rune)
			if r >= 'A' && r <= '_' {
				return []byte{byte(r) ^ 0x40}
			}
		}
		prefix := []byte(nil)
		if ke.Mod&input.ModAlt != 0 {
			prefix = []byte{0x1b}
		}
		return append(prefix, []byte(string(ke.Rune))...)
	}

	if seq, ok := namedKeySequence(ke.Key, ke.Mod); ok {
		return seq
	}
	return nil
}

// modCode is the standard xterm CSI modifier parameter: 1 + bitmask of
// Shift(1)/Alt(2)/Ctrl(4)/Super(8), the exact inverse of
// input/decode.go's modFromCode.
func modCode(m input.Mod) int {
	v := 0
	if m&input.ModShift != 0 {
		v |= 1
	}
	if m&input.ModAlt != 0 {
		v |= 2
	}
	if m&input.ModCtrl != 0 {
		v |= 4
	}
	if m&input.ModSuper != 0 {
		v |= 8
	}
	return v + 1
}

// namedKeySequence encodes a non-rune Key, in the legacy xterm form
// (an unmodified arrow is "ESC [ A"; a modified one is "ESC [ 1 ; N
// A") that input/decode.go's arrowEvent/tildeEvent parse.
func namedKeySequence(k input.Key, mod input.Mod) ([]byte, bool) {
	switch k {
	case input.KeyEnter:
		return []byte{'\r'}, true
	case input.KeyTab:
		if mod&input.ModShift != 0 {
			return []byte("\x1b[Z"), true
		}
		return []byte{'\t'}, true
	case input.KeyBackspace:
		return []byte{0x7f}, true
	case input.KeyEsc:
		return []byte{0x1b}, true
	}

	letter, tilde, ok := keyCode(k)
	if !ok {
		return nil, false
	}
	if mod == 0 {
		if letter != 0 {
			return []byte{0x1b, '[', letter}, true
		}
		return fmt.Appendf(nil, "\x1b[%d~", tilde), true
	}
	if letter != 0 {
		return fmt.Appendf(nil, "\x1b[1;%d%c", modCode(mod), letter), true
	}
	return fmt.Appendf(nil, "\x1b[%d;%d~", tilde, modCode(mod)), true
}

// keyCode returns either the CSI final letter (arrows/Home/End, the
// "ESC [ A"-style sequences) or the tilde-form parameter (Insert/
// Delete/PgUp/PgDn/function keys, "ESC [ N ~"), matching
// interpretCSI/tildeEvent's cases exactly.
func keyCode(k input.Key) (letter byte, tilde int, ok bool) {
	switch k {
	case input.KeyUp:
		return 'A', 0, true
	case input.KeyDown:
		return 'B', 0, true
	case input.KeyRight:
		return 'C', 0, true
	case input.KeyLeft:
		return 'D', 0, true
	case input.KeyHome:
		return 'H', 0, true
	case input.KeyEnd:
		return 'F', 0, true
	case input.KeyInsert:
		return 0, 2, true
	case input.KeyDelete:
		return 0, 3, true
	case input.KeyPgUp:
		return 0, 5, true
	case input.KeyPgDown:
		return 0, 6, true
	case input.KeyF1:
		return 0, 11, true
	case input.KeyF2:
		return 0, 12, true
	case input.KeyF3:
		return 0, 13, true
	case input.KeyF4:
		return 0, 14, true
	case input.KeyF5:
		return 0, 15, true
	case input.KeyF6:
		return 0, 17, true
	case input.KeyF7:
		return 0, 18, true
	case input.KeyF8:
		return 0, 19, true
	case input.KeyF9:
		return 0, 20, true
	case input.KeyF10:
		return 0, 21, true
	case input.KeyF11:
		return 0, 23, true
	case input.KeyF12:
		return 0, 24, true
	default:
		return 0, 0, false
	}
}

// encodeMouse encodes an SGR mouse report (mode 1006), the exact
// inverse of input/decode.go's decodeSGRMouse.
func encodeMouse(m input.MouseEvent) []byte {
	cb := 0
	final := byte('M')
	switch m.Button {
	case input.MouseLeft:
		cb = 0
	case input.MouseMiddle:
		cb = 1
	case input.MouseRight:
		cb = 2
	case input.MouseNone:
		cb = 3
	case input.MouseWheelUp:
		cb = 64
	case input.MouseWheelDown:
		cb = 65
	case input.MouseRelease:
		cb = 3
		final = 'm'
	}
	if m.Mod&input.ModShift != 0 {
		cb |= 4
	}
	if m.Mod&input.ModAlt != 0 {
		cb |= 8
	}
	if m.Mod&input.ModCtrl != 0 {
		cb |= 16
	}
	if m.Drag {
		cb |= 32
	}
	return fmt.Appendf(nil, "\x1b[<%d;%d;%d%c", cb, m.X+1, m.Y+1, final)
}
