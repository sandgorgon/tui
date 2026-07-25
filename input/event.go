package input

import "fmt"

// Event is a decoded input event: KeyEvent, MouseEvent, PasteEvent,
// FocusEvent, or UnknownEvent.
type Event interface {
	isEvent()
}

// Mod is a bitmask of modifier keys held during an event.
type Mod int

const (
	ModShift Mod = 1 << iota
	ModAlt
	ModCtrl
	ModSuper
)

func (m Mod) String() string {
	s := ""
	if m&ModCtrl != 0 {
		s += "Ctrl+"
	}
	if m&ModAlt != 0 {
		s += "Alt+"
	}
	if m&ModShift != 0 {
		s += "Shift+"
	}
	if m&ModSuper != 0 {
		s += "Super+"
	}
	return s
}

// Key names a non-printable or otherwise special key. For printable
// keys, KeyEvent.Rune is set instead and Key is KeyNone.
type Key int

const (
	KeyNone Key = iota
	KeyEnter
	KeyTab
	KeyBackspace
	KeyEsc
	KeyUp
	KeyDown
	KeyRight
	KeyLeft
	KeyHome
	KeyEnd
	KeyPgUp
	KeyPgDown
	KeyInsert
	KeyDelete
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
)

var keyNames = map[Key]string{
	KeyEnter: "Enter", KeyTab: "Tab", KeyBackspace: "Backspace", KeyEsc: "Esc",
	KeyUp: "Up", KeyDown: "Down", KeyRight: "Right", KeyLeft: "Left",
	KeyHome: "Home", KeyEnd: "End", KeyPgUp: "PgUp", KeyPgDown: "PgDown",
	KeyInsert: "Insert", KeyDelete: "Delete",
	KeyF1: "F1", KeyF2: "F2", KeyF3: "F3", KeyF4: "F4",
	KeyF5: "F5", KeyF6: "F6", KeyF7: "F7", KeyF8: "F8",
	KeyF9: "F9", KeyF10: "F10", KeyF11: "F11", KeyF12: "F12",
}

func (k Key) String() string {
	if s, ok := keyNames[k]; ok {
		return s
	}
	return "None"
}

// KeyEvent is a single keypress: either a printable Rune, or a named
// Key for non-printable/special keys, plus any held modifiers.
type KeyEvent struct {
	Rune rune
	Key  Key
	Mod  Mod
}

func (KeyEvent) isEvent() {}

func (e KeyEvent) String() string {
	if e.Key != KeyNone {
		return fmt.Sprintf("%s%s", e.Mod, e.Key)
	}
	return fmt.Sprintf("%s%q", e.Mod, e.Rune)
}

// MouseButton identifies which button a MouseEvent concerns.
type MouseButton int

const (
	MouseNone MouseButton = iota
	MouseLeft
	MouseMiddle
	MouseRight
	MouseWheelUp
	MouseWheelDown
	MouseRelease
)

var mouseButtonNames = map[MouseButton]string{
	MouseLeft: "Left", MouseMiddle: "Middle", MouseRight: "Right",
	MouseWheelUp: "WheelUp", MouseWheelDown: "WheelDown", MouseRelease: "Release",
}

func (b MouseButton) String() string {
	if s, ok := mouseButtonNames[b]; ok {
		return s
	}
	return "None"
}

// MouseEvent is a mouse click, release, wheel tick, or drag, decoded
// from SGR mouse reporting (mode 1006). X and Y are zero-based.
type MouseEvent struct {
	X, Y   int
	Button MouseButton
	Mod    Mod
	Drag   bool
}

func (MouseEvent) isEvent() {}

func (e MouseEvent) String() string {
	drag := ""
	if e.Drag {
		drag = " (drag)"
	}
	return fmt.Sprintf("%s%s@(%d,%d)%s", e.Mod, e.Button, e.X, e.Y, drag)
}

// PasteEvent carries the full text of a bracketed paste (mode 2004).
type PasteEvent struct {
	Text string
}

func (PasteEvent) isEvent() {}

func (e PasteEvent) String() string {
	return fmt.Sprintf("Paste(%q)", e.Text)
}

// FocusEvent reports the terminal gaining or losing focus (mode 1004).
type FocusEvent struct {
	Focused bool
}

func (FocusEvent) isEvent() {}

func (e FocusEvent) String() string {
	if e.Focused {
		return "FocusIn"
	}
	return "FocusOut"
}

// UnknownEvent carries the raw bytes of a recognized-but-unhandled or
// unrecognized escape sequence, so callers can inspect or ignore it
// rather than the decoder erroring out.
type UnknownEvent struct {
	Raw []byte
}

func (UnknownEvent) isEvent() {}

func (e UnknownEvent) String() string {
	return fmt.Sprintf("Unknown(% x)", e.Raw)
}
