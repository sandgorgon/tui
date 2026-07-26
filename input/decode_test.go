package input

import (
	"net"
	"os"
	"reflect"
	"testing"
	"time"
)

// decodeN writes input on one end of an in-memory net.Pipe (which
// supports SetReadDeadline, exactly like a real terminal fd) and
// decodes n events from the other end.
func decodeN(t *testing.T, input []byte, n int) []Event {
	t.Helper()
	server, client := net.Pipe()
	t.Cleanup(func() { server.Close(); client.Close() })

	go client.Write(input)

	d := NewDecoder(server)
	events := make([]Event, 0, n)
	for i := range n {
		ev, err := d.Decode()
		if err != nil {
			t.Fatalf("Decode #%d: %v", i, err)
		}
		events = append(events, ev)
	}
	return events
}

func decode1(t *testing.T, input []byte) Event {
	t.Helper()
	return decodeN(t, input, 1)[0]
}

func TestDecodeBasicKeys(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  Event
	}{
		{"printable ascii", []byte("a"), KeyEvent{Rune: 'a'}},
		{"ctrl-c", []byte{0x03}, KeyEvent{Rune: 'c', Mod: ModCtrl}},
		{"ctrl-a", []byte{0x01}, KeyEvent{Rune: 'a', Mod: ModCtrl}},
		{"tab", []byte{0x09}, KeyEvent{Key: KeyTab}},
		{"enter/cr", []byte{0x0d}, KeyEvent{Key: KeyEnter}},
		{"backspace del", []byte{0x7f}, KeyEvent{Key: KeyBackspace}},
		{"backspace bs", []byte{0x08}, KeyEvent{Key: KeyBackspace}},
		{"utf8 rune", []byte("é"), KeyEvent{Rune: 'é'}},
		{"utf8 emoji", []byte("😀"), KeyEvent{Rune: '😀'}},
		{"arrow up", []byte("\x1b[A"), KeyEvent{Key: KeyUp}},
		{"arrow down", []byte("\x1b[B"), KeyEvent{Key: KeyDown}},
		{"ctrl+up", []byte("\x1b[1;5A"), KeyEvent{Key: KeyUp, Mod: ModCtrl}},
		{"shift+up", []byte("\x1b[1;2A"), KeyEvent{Key: KeyUp, Mod: ModShift}},
		{"shift-tab", []byte("\x1b[Z"), KeyEvent{Key: KeyTab, Mod: ModShift}},
		{"home", []byte("\x1b[H"), KeyEvent{Key: KeyHome}},
		{"end", []byte("\x1b[F"), KeyEvent{Key: KeyEnd}},
		{"insert tilde", []byte("\x1b[2~"), KeyEvent{Key: KeyInsert}},
		{"delete tilde", []byte("\x1b[3~"), KeyEvent{Key: KeyDelete}},
		{"pgup tilde", []byte("\x1b[5~"), KeyEvent{Key: KeyPgUp}},
		{"f5 tilde", []byte("\x1b[15~"), KeyEvent{Key: KeyF5}},
		{"f12 tilde", []byte("\x1b[24~"), KeyEvent{Key: KeyF12}},
		{"ss3 f1", []byte("\x1bOP"), KeyEvent{Key: KeyF1}},
		{"ss3 up", []byte("\x1bOA"), KeyEvent{Key: KeyUp}},
		{"csi-u ctrl+a", []byte("\x1b[97;5u"), KeyEvent{Rune: 'a', Mod: ModCtrl}},
		{"focus in", []byte("\x1b[I"), FocusEvent{Focused: true}},
		{"focus out", []byte("\x1b[O"), FocusEvent{Focused: false}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decode1(t, tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("decode(%q) = %#v, want %#v", tt.input, got, tt.want)
			}
		})
	}
}

func TestDecodeMouse(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  Event
	}{
		{"left press", []byte("\x1b[<0;10;20M"), MouseEvent{X: 9, Y: 19, Button: MouseLeft}},
		{"left release", []byte("\x1b[<0;10;20m"), MouseEvent{X: 9, Y: 19, Button: MouseRelease}},
		{"right press with ctrl", []byte("\x1b[<18;5;5M"), MouseEvent{X: 4, Y: 4, Button: MouseRight, Mod: ModCtrl}},
		{"wheel up", []byte("\x1b[<64;1;1M"), MouseEvent{X: 0, Y: 0, Button: MouseWheelUp}},
		{"wheel down", []byte("\x1b[<65;1;1M"), MouseEvent{X: 0, Y: 0, Button: MouseWheelDown}},
		{"drag", []byte("\x1b[<32;3;4M"), MouseEvent{X: 2, Y: 3, Button: MouseLeft, Drag: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decode1(t, tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("decode(%q) = %#v, want %#v", tt.input, got, tt.want)
			}
		})
	}
}

func TestDecodeBracketedPaste(t *testing.T) {
	// The stray "\x1b[" mid-paste is a false start against the "\x1b[201~"
	// terminator (matches its first two bytes, then diverges at 'w') and
	// must be flushed back as literal content, not silently dropped.
	input := []byte("\x1b[200~hello\x1b[world\x1b[201~")
	want := PasteEvent{Text: "hello\x1b[world"}

	got := decode1(t, input)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("decode(%q) = %#v, want %#v", input, got, want)
	}
}

func TestDecodeUnknownCSI(t *testing.T) {
	got := decode1(t, []byte("\x1b[99z"))
	u, ok := got.(UnknownEvent)
	if !ok {
		t.Fatalf("decode = %#v (%T), want UnknownEvent", got, got)
	}
	if string(u.Raw) != "\x1b[99z" {
		t.Errorf("UnknownEvent.Raw = %q, want %q", u.Raw, "\x1b[99z")
	}
}

func TestDecodeEscAlone(t *testing.T) {
	server, client := net.Pipe()
	t.Cleanup(func() { server.Close(); client.Close() })
	go client.Write([]byte{0x1b})

	d := NewDecoder(server)
	d.SetEscTimeout(20 * time.Millisecond)

	ev, err := d.Decode()
	if err != nil {
		t.Fatal(err)
	}
	want := KeyEvent{Key: KeyEsc}
	if !reflect.DeepEqual(ev, want) {
		t.Errorf("decode(ESC alone) = %#v, want %#v", ev, want)
	}
}

func TestDecodeAltKey(t *testing.T) {
	server, client := net.Pipe()
	t.Cleanup(func() { server.Close(); client.Close() })
	go func() {
		client.Write([]byte{0x1b})
		time.Sleep(5 * time.Millisecond)
		client.Write([]byte("x"))
	}()

	d := NewDecoder(server)
	d.SetEscTimeout(50 * time.Millisecond)

	ev, err := d.Decode()
	if err != nil {
		t.Fatal(err)
	}
	want := KeyEvent{Rune: 'x', Mod: ModAlt}
	if !reflect.DeepEqual(ev, want) {
		t.Errorf("decode(Alt+x, delayed) = %#v, want %#v", ev, want)
	}
}

func TestDecodeSlowCSIStillRecognized(t *testing.T) {
	server, client := net.Pipe()
	t.Cleanup(func() { server.Close(); client.Close() })
	go func() {
		client.Write([]byte{0x1b})
		time.Sleep(5 * time.Millisecond)
		client.Write([]byte("[A"))
	}()

	d := NewDecoder(server)
	d.SetEscTimeout(50 * time.Millisecond)

	ev, err := d.Decode()
	if err != nil {
		t.Fatal(err)
	}
	want := KeyEvent{Key: KeyUp}
	if !reflect.DeepEqual(ev, want) {
		t.Errorf("decode(slow arrow-up) = %#v, want %#v", ev, want)
	}
}

func TestDecodeSequenceOfEvents(t *testing.T) {
	input := []byte("ab\x1b[A\r")
	want := []Event{
		KeyEvent{Rune: 'a'},
		KeyEvent{Rune: 'b'},
		KeyEvent{Key: KeyUp},
		KeyEvent{Key: KeyEnter},
	}
	got := decodeN(t, input, len(want))
	if !reflect.DeepEqual(got, want) {
		t.Errorf("decodeN(%q) = %#v, want %#v", input, got, want)
	}
}

// noDeadlineReader wraps a real reader but fails every SetReadDeadline
// call with os.ErrNoDeadline, standing in for a pty/tty that doesn't
// support read deadlines at all (observed in practice via a sandboxed
// nested-pty test harness for examples/gallery, M12 — see
// docs/DESIGN.md §9).
type noDeadlineReader struct{ r reader }

func (n noDeadlineReader) Read(p []byte) (int, error)      { return n.r.Read(p) }
func (n noDeadlineReader) SetReadDeadline(time.Time) error { return os.ErrNoDeadline }

// TestDecodeEscAloneWithoutDeadlineSupportDoesNotError is a regression
// test for a real crash: readByteTimeout used to propagate
// SetReadDeadline's error straight out of Decode, so on a reader that
// can't support deadlines, a single standalone Escape keypress made
// Decode return a fatal error — which killed the whole App (confirmed
// against the already-shipped examples/todo, not something introduced
// by gallery). It must instead behave like an immediate timeout: still
// report KeyEsc, no error.
func TestDecodeEscAloneWithoutDeadlineSupportDoesNotError(t *testing.T) {
	server, client := net.Pipe()
	t.Cleanup(func() { server.Close(); client.Close() })
	go client.Write([]byte{0x1b})

	d := NewDecoder(noDeadlineReader{r: server})
	d.SetEscTimeout(20 * time.Millisecond)

	ev, err := d.Decode()
	if err != nil {
		t.Fatalf("Decode returned an error instead of degrading gracefully: %v", err)
	}
	want := KeyEvent{Key: KeyEsc}
	if !reflect.DeepEqual(ev, want) {
		t.Errorf("decode(ESC alone, no deadline support) = %#v, want %#v", ev, want)
	}
}

func TestDecodeEOF(t *testing.T) {
	server, client := net.Pipe()
	client.Close()
	t.Cleanup(func() { server.Close() })

	d := NewDecoder(server)
	if _, err := d.Decode(); err == nil {
		t.Error("Decode on a closed pipe: got nil error, want non-nil")
	}
}
