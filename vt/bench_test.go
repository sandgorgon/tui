package vt

import "testing"

// benchSeq approximates a burst of real program output (colored
// prompt, cursor moves, an alternate-screen redraw) rather than plain
// text, since SGR/CSI dispatch — not Print — is where a slow parser
// would actually show up.
var benchSeq = []byte("\x1b[?1049h\x1b[2J\x1b[H" +
	"\x1b[1;1H\x1b[7m CPU  MEM  TASKS \x1b[0m" +
	"\x1b[2;1H\x1b[32mproc1\x1b[0m running with a fairly long line of plain text here\r\n" +
	"\x1b[38;2;10;20;30mtruecolor text\x1b[0m\r\n" +
	"\x1b[3;3r\x1b[3;1Hscrolled region content\n\n\n" +
	"\x1b]0;window title\x07" +
	"café 日本語 line\r\n")

// BenchmarkParserFeed feeds benchSeq into a fresh Screen repeatedly,
// simulating a long-running program's continuous output — the state
// machine's steady-state throughput, not just a cold start.
func BenchmarkParserFeed(b *testing.B) {
	s := NewScreen(80, 24)
	p := NewParser()
	for b.Loop() {
		p.Feed(benchSeq, s)
	}
}
