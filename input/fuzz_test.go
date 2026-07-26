package input

import (
	"net"
	"testing"
	"time"
)

// FuzzDecode feeds arbitrary byte streams through Decoder.Decode,
// looking for panics or hangs rather than specific event shapes (those
// are covered by the table-driven tests in decode_test.go). The
// escape timeout is cut way down from DefaultEscTimeout so a fuzz
// input full of lone ESC/CSI-prefix bytes (which legitimately makes
// Decode block waiting for a follow-up byte that will never arrive)
// doesn't make each fuzz iteration slow; the decode loop is also
// capped so a decoder bug that never advances the read position can't
// hang the fuzzer forever — it fails the test instead.
func FuzzDecode(f *testing.F) {
	seeds := []string{
		"",
		"a",
		"\x03",
		"\x1b",
		"\x1b[",
		"\x1b[A",
		"\x1b[1;5A",
		"\x1b[Z",
		"\x1b[H",
		"\x1b[2~",
		"\x1b[15~",
		"\x1bOP",
		"\x1bOA",
		"\x1b[97;5u",
		"\x1b[I",
		"\x1b[O",
		"\x1b[<0;10;20M",
		"\x1b[<32;10;20M",
		"\x1b[<64;10;20M",
		"\x1b[200~pasted text\x1b[201~",
		"\x1b[200~unterminated paste",
		"é",
		"😀",
		string([]byte{0xff, 0xfe}),
		string([]byte{0x1b, '['}),
		"\x7f",
		"\x08",
		"\x09",
		"\x0d",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		server, client := net.Pipe()
		defer server.Close()
		defer client.Close()

		go func() {
			client.Write(data)
			client.Close()
		}()

		d := NewDecoder(server)
		d.SetEscTimeout(time.Millisecond)

		const maxEvents = 10000
		for range maxEvents {
			if _, err := d.Decode(); err != nil {
				return
			}
		}
		t.Fatalf("did not terminate within %d events for input %q", maxEvents, data)
	})
}
