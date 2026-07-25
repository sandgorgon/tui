package input

import (
	"errors"
	"net"
	"os"
	"strconv"
	"time"
	"unicode/utf8"
)

// reader is what a Decoder reads from: a byte stream that also supports
// read deadlines, needed to disambiguate a lone Escape keypress from the
// start of an escape sequence or an Alt+key chord (see decodeEscape).
// *os.File and net.Conn (including net.Pipe, used in tests) both satisfy
// this.
type reader interface {
	Read(p []byte) (int, error)
	SetReadDeadline(t time.Time) error
}

// DefaultEscTimeout is how long Decoder waits, after seeing a lone ESC
// byte, for a follow-up byte before concluding it was a standalone
// Escape keypress rather than the start of a sequence.
const DefaultEscTimeout = 50 * time.Millisecond

// Decoder turns a raw terminal input byte stream into a sequence of
// Events: keys (including CSI-u / kitty keyboard protocol and legacy
// xterm sequences), SGR mouse reports, bracketed paste, and focus
// in/out. See docs/DESIGN.md §4.
type Decoder struct {
	r          reader
	escTimeout time.Duration
	buf        []byte
	pos        int
}

// NewDecoder returns a Decoder reading from r, which must support read
// deadlines (an *os.File on a real terminal or pty, or a net.Conn such
// as net.Pipe in tests).
func NewDecoder(r reader) *Decoder {
	return &Decoder{r: r, escTimeout: DefaultEscTimeout}
}

// SetEscTimeout overrides DefaultEscTimeout.
func (d *Decoder) SetEscTimeout(t time.Duration) { d.escTimeout = t }

// Decode reads and returns the next Event, blocking until one is
// available or the underlying reader returns an error (including io.EOF).
func (d *Decoder) Decode() (Event, error) {
	b, err := d.readByte()
	if err != nil {
		return nil, err
	}
	return d.dispatch(b)
}

func (d *Decoder) dispatch(b byte) (Event, error) {
	switch {
	case b == 0x1b:
		return d.decodeEscape()
	case b < 0x20:
		return decodeControl(b), nil
	case b == 0x7f:
		return KeyEvent{Key: KeyBackspace}, nil
	default:
		return d.decodeRune(b)
	}
}

func decodeControl(b byte) KeyEvent {
	switch b {
	case 0x09:
		return KeyEvent{Key: KeyTab}
	case 0x0d:
		return KeyEvent{Key: KeyEnter}
	case 0x08:
		return KeyEvent{Key: KeyBackspace}
	default:
		// The standard terminal convention: Ctrl+<letter> is transmitted
		// as <letter> & 0x1F, equivalently <letter> ^ 0x40 for bytes in
		// this range. This also naturally covers Ctrl+\, Ctrl+], Ctrl+^,
		// Ctrl+_ and Ctrl+@ (NUL).
		r := rune(b ^ 0x40)
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		return KeyEvent{Rune: r, Mod: ModCtrl}
	}
}

func (d *Decoder) decodeRune(b byte) (Event, error) {
	if b < 0x80 {
		return KeyEvent{Rune: rune(b)}, nil
	}
	n := utf8SeqLen(b)
	if n <= 1 {
		return KeyEvent{Rune: utf8.RuneError}, nil
	}
	buf := make([]byte, n)
	buf[0] = b
	for i := 1; i < n; i++ {
		nb, err := d.readByte()
		if err != nil {
			return KeyEvent{Rune: utf8.RuneError}, nil
		}
		buf[i] = nb
	}
	r, _ := utf8.DecodeRune(buf)
	return KeyEvent{Rune: r}, nil
}

func utf8SeqLen(b byte) int {
	switch {
	case b&0xE0 == 0xC0:
		return 2
	case b&0xF0 == 0xE0:
		return 3
	case b&0xF8 == 0xF0:
		return 4
	default:
		return 1
	}
}

// decodeEscape handles the byte after a lone ESC (0x1b): '[' starts a
// CSI sequence, 'O' starts an SS3 sequence, nothing arriving within
// escTimeout means the user pressed Escape by itself, and anything else
// is Alt+<key> (Alt is conventionally sent as ESC followed by the key's
// normal encoding).
func (d *Decoder) decodeEscape() (Event, error) {
	b, ok, err := d.readByteTimeout(d.escTimeout)
	if err != nil {
		return nil, err
	}
	if !ok {
		return KeyEvent{Key: KeyEsc}, nil
	}
	switch b {
	case '[':
		return d.decodeCSI()
	case 'O':
		return d.decodeSS3()
	default:
		ev, err := d.dispatch(b)
		if err != nil {
			return nil, err
		}
		if ke, ok := ev.(KeyEvent); ok {
			ke.Mod |= ModAlt
			return ke, nil
		}
		return ev, nil
	}
}

func (d *Decoder) decodeSS3() (Event, error) {
	b, err := d.readByte()
	if err != nil {
		return nil, err
	}
	switch b {
	case 'A':
		return KeyEvent{Key: KeyUp}, nil
	case 'B':
		return KeyEvent{Key: KeyDown}, nil
	case 'C':
		return KeyEvent{Key: KeyRight}, nil
	case 'D':
		return KeyEvent{Key: KeyLeft}, nil
	case 'H':
		return KeyEvent{Key: KeyHome}, nil
	case 'F':
		return KeyEvent{Key: KeyEnd}, nil
	case 'P':
		return KeyEvent{Key: KeyF1}, nil
	case 'Q':
		return KeyEvent{Key: KeyF2}, nil
	case 'R':
		return KeyEvent{Key: KeyF3}, nil
	case 'S':
		return KeyEvent{Key: KeyF4}, nil
	default:
		return UnknownEvent{Raw: []byte{0x1b, 'O', b}}, nil
	}
}

// decodeCSI parses "ESC [" params... intermediates... final, per ECMA-48:
// parameter bytes are 0x30-0x3F, intermediate bytes 0x20-0x2F, and the
// sequence ends with a final byte 0x40-0x7E.
func (d *Decoder) decodeCSI() (Event, error) {
	var params []byte
	b, err := d.readByte()
	if err != nil {
		return nil, err
	}
	for b >= 0x30 && b <= 0x3F {
		params = append(params, b)
		if b, err = d.readByte(); err != nil {
			return nil, err
		}
	}
	var intermediates []byte
	for b >= 0x20 && b <= 0x2F {
		intermediates = append(intermediates, b)
		if b, err = d.readByte(); err != nil {
			return nil, err
		}
	}
	return d.interpretCSI(params, intermediates, b)
}

func (d *Decoder) interpretCSI(params, intermediates []byte, final byte) (Event, error) {
	private := byte(0)
	if len(params) > 0 && isPrivateMarker(params[0]) {
		private = params[0]
		params = params[1:]
	}
	ps := splitParams(params)

	if private == '<' && (final == 'M' || final == 'm') {
		return decodeSGRMouse(ps, final), nil
	}

	if final == '~' && len(ps) >= 1 && ps[0] == 200 && private == 0 {
		return d.decodePasteBody()
	}

	switch final {
	case 'A':
		return arrowEvent(KeyUp, ps), nil
	case 'B':
		return arrowEvent(KeyDown, ps), nil
	case 'C':
		return arrowEvent(KeyRight, ps), nil
	case 'D':
		return arrowEvent(KeyLeft, ps), nil
	case 'H':
		return arrowEvent(KeyHome, ps), nil
	case 'F':
		return arrowEvent(KeyEnd, ps), nil
	case 'Z':
		return KeyEvent{Key: KeyTab, Mod: ModShift}, nil
	case 'I':
		return FocusEvent{Focused: true}, nil
	case 'O':
		return FocusEvent{Focused: false}, nil
	case 'u':
		return decodeCSIu(ps), nil
	case '~':
		return tildeEvent(ps), nil
	}
	return UnknownEvent{Raw: buildRaw(private, params, intermediates, final)}, nil
}

func isPrivateMarker(b byte) bool {
	return b == '?' || b == '<' || b == '=' || b == '>'
}

func buildRaw(private byte, params, intermediates []byte, final byte) []byte {
	raw := []byte{0x1b, '['}
	if private != 0 {
		raw = append(raw, private)
	}
	raw = append(raw, params...)
	raw = append(raw, intermediates...)
	raw = append(raw, final)
	return raw
}

// decodePasteBody streams bytes until the literal bracketed-paste
// terminator "ESC [ 201 ~", returning everything before it as one
// PasteEvent. Paste content can contain arbitrary bytes (including ones
// that would otherwise start new escape sequences), so it must not go
// through the normal per-event dispatch.
func (d *Decoder) decodePasteBody() (Event, error) {
	const term = "\x1b[201~"
	var buf []byte
	match := 0
	for {
		b, err := d.readByte()
		if err != nil {
			return PasteEvent{Text: string(buf)}, err
		}
		if b == term[match] {
			match++
			if match == len(term) {
				return PasteEvent{Text: string(buf)}, nil
			}
			continue
		}
		if match > 0 {
			buf = append(buf, term[:match]...)
			match = 0
			if b == term[0] {
				match = 1
				continue
			}
		}
		buf = append(buf, b)
	}
}

func arrowEvent(key Key, ps []int) Event {
	return KeyEvent{Key: key, Mod: modFromParam(ps, 1)}
}

func tildeEvent(ps []int) Event {
	if len(ps) == 0 {
		return UnknownEvent{}
	}
	var key Key
	switch ps[0] {
	case 2:
		key = KeyInsert
	case 3:
		key = KeyDelete
	case 5:
		key = KeyPgUp
	case 6:
		key = KeyPgDown
	case 1, 7:
		key = KeyHome
	case 4, 8:
		key = KeyEnd
	case 11:
		key = KeyF1
	case 12:
		key = KeyF2
	case 13:
		key = KeyF3
	case 14:
		key = KeyF4
	case 15:
		key = KeyF5
	case 17:
		key = KeyF6
	case 18:
		key = KeyF7
	case 19:
		key = KeyF8
	case 20:
		key = KeyF9
	case 21:
		key = KeyF10
	case 23:
		key = KeyF11
	case 24:
		key = KeyF12
	default:
		return UnknownEvent{}
	}
	return KeyEvent{Key: key, Mod: modFromParam(ps, 1)}
}

// decodeCSIu decodes the base form of the CSI-u / kitty keyboard
// protocol: "CSI unicode-key-code ; modifiers u". Extended kitty fields
// (alternate/shifted codepoints, explicit event type) are intentionally
// not parsed — a documented, scoped simplification (see docs/DESIGN.md
// §7's VT-conformance scope note; the same "common subset, not every
// protocol detail" philosophy applies here).
func decodeCSIu(ps []int) Event {
	if len(ps) == 0 {
		return UnknownEvent{}
	}
	return KeyEvent{Rune: rune(ps[0]), Mod: modFromParam(ps, 1)}
}

func decodeSGRMouse(ps []int, final byte) Event {
	if len(ps) < 3 {
		return UnknownEvent{}
	}
	cb, x, y := ps[0], ps[1], ps[2]

	mod := Mod(0)
	if cb&4 != 0 {
		mod |= ModShift
	}
	if cb&8 != 0 {
		mod |= ModAlt
	}
	if cb&16 != 0 {
		mod |= ModCtrl
	}
	drag := cb&32 != 0
	release := final == 'm'

	var button MouseButton
	switch {
	case cb&64 != 0:
		if cb&1 != 0 {
			button = MouseWheelDown
		} else {
			button = MouseWheelUp
		}
	default:
		switch cb & 3 {
		case 0:
			button = MouseLeft
		case 1:
			button = MouseMiddle
		case 2:
			button = MouseRight
		case 3:
			button = MouseNone
		}
	}
	if release {
		button = MouseRelease
	}

	return MouseEvent{X: x - 1, Y: y - 1, Button: button, Mod: mod, Drag: drag}
}

func modFromCode(code int) Mod {
	if code < 1 {
		return 0
	}
	v := code - 1
	var m Mod
	if v&1 != 0 {
		m |= ModShift
	}
	if v&2 != 0 {
		m |= ModAlt
	}
	if v&4 != 0 {
		m |= ModCtrl
	}
	if v&8 != 0 {
		m |= ModSuper
	}
	return m
}

func modFromParam(ps []int, idx int) Mod {
	if len(ps) > idx {
		return modFromCode(ps[idx])
	}
	return 0
}

// splitParams splits CSI parameter bytes on ';'. Where a field itself
// contains a kitty-style ':' sub-parameter, only the portion before the
// ':' is used (see decodeCSIu). Empty fields decode as 0.
func splitParams(b []byte) []int {
	if len(b) == 0 {
		return nil
	}
	var out []int
	start := 0
	for i := 0; i <= len(b); i++ {
		if i == len(b) || b[i] == ';' {
			field := b[start:i]
			if j := indexByte(field, ':'); j >= 0 {
				field = field[:j]
			}
			n, _ := strconv.Atoi(string(field))
			out = append(out, n)
			start = i + 1
		}
	}
	return out
}

func indexByte(b []byte, c byte) int {
	for i, x := range b {
		if x == c {
			return i
		}
	}
	return -1
}

func (d *Decoder) fill() error {
	if d.pos < len(d.buf) {
		return nil
	}
	tmp := make([]byte, 256)
	n, err := d.r.Read(tmp)
	if n > 0 {
		d.buf = tmp[:n]
		d.pos = 0
		return nil
	}
	if err != nil {
		return err
	}
	return nil
}

func (d *Decoder) readByte() (byte, error) {
	if err := d.fill(); err != nil {
		return 0, err
	}
	b := d.buf[d.pos]
	d.pos++
	return b, nil
}

// readByteTimeout tries to read one more byte within timeout. ok is
// false (with a nil error) if the deadline passed with nothing arriving.
func (d *Decoder) readByteTimeout(timeout time.Duration) (b byte, ok bool, err error) {
	if d.pos < len(d.buf) {
		b := d.buf[d.pos]
		d.pos++
		return b, true, nil
	}
	if err := d.r.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return 0, false, err
	}
	defer d.r.SetReadDeadline(time.Time{})

	tmp := make([]byte, 256)
	n, err := d.r.Read(tmp)
	if n > 0 {
		d.buf = tmp[:n]
		d.pos = 1
		return tmp[0], true, nil
	}
	if err != nil {
		if isTimeout(err) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return 0, false, nil
}

func isTimeout(err error) bool {
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	if ne, ok := errors.AsType[net.Error](err); ok {
		return ne.Timeout()
	}
	return false
}
