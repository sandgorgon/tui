package term

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestWriteClipboardFormat(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteClipboard(&buf, "hello"); err != nil {
		t.Fatalf("WriteClipboard: %v", err)
	}

	want := "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte("hello")) + "\x07"
	if got := buf.String(); got != want {
		t.Errorf("WriteClipboard output = %q, want %q", got, want)
	}
}

func TestWriteClipboardEmptyString(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteClipboard(&buf, ""); err != nil {
		t.Fatalf("WriteClipboard: %v", err)
	}
	if got, want := buf.String(), "\x1b]52;c;\x07"; got != want {
		t.Errorf("WriteClipboard(\"\") = %q, want %q", got, want)
	}
}

func TestWriteClipboardBinarySafeViaBase64(t *testing.T) {
	// Text containing bytes that would otherwise be misinterpreted as
	// control sequences (ESC, BEL itself) must survive intact through
	// the base64 encoding.
	tricky := "line1\nline2\x1b[31mred\x07bell"
	var buf bytes.Buffer
	if err := WriteClipboard(&buf, tricky); err != nil {
		t.Fatalf("WriteClipboard: %v", err)
	}

	want := "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(tricky)) + "\x07"
	if got := buf.String(); got != want {
		t.Errorf("WriteClipboard output = %q, want %q", got, want)
	}

	// And round-trips back to the original text.
	prefix := "\x1b]52;c;"
	got := buf.String()
	encoded := got[len(prefix) : len(got)-1] // strip prefix and trailing BEL
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	if string(decoded) != tricky {
		t.Errorf("decoded = %q, want %q", decoded, tricky)
	}
}
