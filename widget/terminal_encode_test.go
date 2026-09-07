package widget

import (
	"net"
	"testing"
	"time"

	"github.com/sandgorgon/tui/input"
)

// decodeOne feeds b through a real input.Decoder via a net.Pipe (the
// same trick input's own tests and its doc comment recommend) and
// returns the single Event it decodes.
func decodeOne(t *testing.T, b []byte) input.Event {
	t.Helper()
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		client.Write(b)
	}()

	dec := input.NewDecoder(server)
	ev, err := dec.Decode()
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("write side never finished")
	}
	return ev
}

func TestEncodeEventRoundTripsPrintableRunes(t *testing.T) {
	for _, r := range []rune{'a', 'Z', '5', ' ', '中'} {
		want := input.KeyEvent{Rune: r}
		got := decodeOne(t, encodeEvent(want, false))
		if got != input.Event(want) {
			t.Errorf("rune %q: decoded %v, want %v", r, got, want)
		}
	}
}

func TestEncodeEventRoundTripsCtrlLetter(t *testing.T) {
	want := input.KeyEvent{Rune: 'a', Mod: input.ModCtrl}
	got := decodeOne(t, encodeEvent(want, false))
	if got != input.Event(want) {
		t.Errorf("decoded %v, want %v", got, want)
	}
}

func TestEncodeEventRoundTripsNamedKeys(t *testing.T) {
	tests := []input.KeyEvent{
		{Key: input.KeyEnter},
		{Key: input.KeyTab},
		{Key: input.KeyBackspace},
		{Key: input.KeyUp},
		{Key: input.KeyDown},
		{Key: input.KeyLeft},
		{Key: input.KeyRight},
		{Key: input.KeyHome},
		{Key: input.KeyEnd},
		{Key: input.KeyDelete},
		{Key: input.KeyInsert},
		{Key: input.KeyPgUp},
		{Key: input.KeyPgDown},
		{Key: input.KeyF1},
		{Key: input.KeyF5},
		{Key: input.KeyF12},
	}
	for _, want := range tests {
		got := decodeOne(t, encodeEvent(want, false))
		if got != input.Event(want) {
			t.Errorf("key %v: decoded %v, want %v", want.Key, got, want)
		}
	}
}

func TestEncodeEventRoundTripsModifiedArrow(t *testing.T) {
	want := input.KeyEvent{Key: input.KeyUp, Mod: input.ModCtrl | input.ModShift}
	got := decodeOne(t, encodeEvent(want, false))
	if got != input.Event(want) {
		t.Errorf("decoded %v, want %v", got, want)
	}
}

func TestEncodeEventRoundTripsShiftTab(t *testing.T) {
	want := input.KeyEvent{Key: input.KeyTab, Mod: input.ModShift}
	got := decodeOne(t, encodeEvent(want, false))
	if got != input.Event(want) {
		t.Errorf("decoded %v, want %v", got, want)
	}
}

// TestEncodeEventRoundTripsAppCursorKeys covers DECCKM (application
// cursor-key mode): an unmodified arrow/Home/End must switch from the
// legacy CSI form ("ESC [ A") to the SS3 form ("ESC O A") that
// input/decode.go's decodeSS3 parses, while a modified arrow keeps
// using CSI regardless (SS3 has no room for a modifier parameter — see
// namedKeySequence's doc comment).
func TestEncodeEventRoundTripsAppCursorKeys(t *testing.T) {
	unmodified := []input.KeyEvent{
		{Key: input.KeyUp}, {Key: input.KeyDown},
		{Key: input.KeyLeft}, {Key: input.KeyRight},
		{Key: input.KeyHome}, {Key: input.KeyEnd},
	}
	for _, want := range unmodified {
		b := encodeEvent(want, true)
		if len(b) < 2 || b[0] != 0x1b || b[1] != 'O' {
			t.Errorf("key %v: encoded %q, want SS3 (ESC O ...) form", want.Key, b)
		}
		if got := decodeOne(t, b); got != input.Event(want) {
			t.Errorf("key %v: decoded %v, want %v", want.Key, got, want)
		}
	}

	modified := input.KeyEvent{Key: input.KeyUp, Mod: input.ModCtrl}
	b := encodeEvent(modified, true)
	if len(b) < 2 || b[0] != 0x1b || b[1] != '[' {
		t.Errorf("modified key with appCursorKeys: encoded %q, want CSI (ESC [ ...) form", b)
	}
	if got := decodeOne(t, b); got != input.Event(modified) {
		t.Errorf("decoded %v, want %v", got, modified)
	}
}

func TestEncodeEventRoundTripsMouse(t *testing.T) {
	tests := []input.MouseEvent{
		{X: 10, Y: 5, Button: input.MouseLeft},
		{X: 0, Y: 0, Button: input.MouseRight, Mod: input.ModShift},
		{X: 3, Y: 3, Button: input.MouseLeft, Drag: true},
		{X: 1, Y: 1, Button: input.MouseWheelUp},
		{X: 1, Y: 1, Button: input.MouseWheelDown},
		{X: 2, Y: 2, Button: input.MouseRelease},
	}
	for _, want := range tests {
		got := decodeOne(t, encodeEvent(want, false))
		if got != input.Event(want) {
			t.Errorf("mouse %+v: decoded %v, want %v", want, got, want)
		}
	}
}

func TestEncodeEventPaste(t *testing.T) {
	want := input.PasteEvent{Text: "hello\nworld"}
	got := decodeOne(t, encodeEvent(want, false))
	if got != input.Event(want) {
		t.Errorf("decoded %v, want %v", got, want)
	}
}
