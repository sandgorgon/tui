package vt

import "testing"

// FuzzParser feeds arbitrary byte streams through Parser into a real
// Screen (not just the bare recorder from parser_test.go), so the fuzz
// corpus exercises the state machine and screen semantics (scroll
// regions, alt-screen, SGR, OSC handling) together — the failure mode
// worth catching here is a panic or an infinite loop, not a specific
// screen content assertion. Seeds are drawn from the hand-written
// conformance fixtures (conformance_test.go, parser_test.go,
// screen_test.go) covering the sequences vim/htop/tmux/less actually
// emit, plus a few deliberately malformed ones that already have
// regression tests (CAN/SUB abort, DEL mid-params, stray ESC mid-OSC).
func FuzzParser(f *testing.F) {
	seeds := []string{
		"",
		"plain ASCII text\r\n",
		"\x1b[?1049h\x1b[2J\x1b[H",
		"\x1b[?1049l",
		"\x1b[1;1H\x1b[7m CPU  MEM \x1b[0m",
		"\x1b[32mgreen\x1b[0m",
		"\x1b[38;2;10;20;30mtruecolor\x1b[0m",
		"\x1b[38;5;200m256color\x1b[0m",
		"\x1b[1;3r",           // set scroll region
		"\x1b[2;4r\n\n\n\n\n", // scroll region + scroll
		"\x1b[6n",             // DSR
		"\x1b[c",              // DA1
		"\x1b[?1;2c",          // DA1 response shape fed back in
		"\x1b[3;13R",          // CPR
		"\x1b[?25h\x1b[?25l",  // cursor visibility
		"\x1b[?7h\x1b[?7l",    // autowrap on/off
		"\x1b7\x1b8",          // DECSC/DECRC
		"\x1bOP",              // SS3
		"\x1b]0;title\x07",    // OSC window title, BEL-terminated
		"\x1b]8;;http://example.com\x1b\\link text\x1b]8;;\x1b\\",
		"\x1b]52;c;aGVsbG8=\x07", // OSC 52 clipboard
		"\x1b^ignored pm string\x1b\\A",
		"\x1b]0;abc\x1b[1mX",
		"\x1b[1\x7f5A",
		"\x1b[1\x7f5AX",
		"\x1b[18;A", // CAN/SUB style abort variants
		"\x18\x1a",
		"café 日本語 😀",
		"\x1b[200~pasted\x1b[201~",
		string([]byte{0x1b, '[', 0xff, 0xfe}),
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		s := NewScreen(80, 24)
		p := NewParser()
		p.Feed(data, s)
	})
}
