package term

import (
	"io"
	"os"
	"testing"
	"time"
)

func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestDetectEnv(t *testing.T) {
	tests := []struct {
		name string
		vars map[string]string
		want Capabilities
	}{
		{
			name: "xterm-256color",
			vars: map[string]string{"TERM": "xterm-256color"},
			want: Capabilities{Name: "xterm-256color", ColorLevel: ColorTrueColor, Mouse: true, BracketedPaste: true, FocusEvents: true},
		},
		{
			name: "tmux inside 256color TERM",
			vars: map[string]string{"TERM": "tmux-256color"},
			want: Capabilities{Name: "tmux-256color", ColorLevel: ColorTrueColor, Mouse: true, BracketedPaste: true, FocusEvents: true, SynchronizedOutput: true},
		},
		{
			name: "unknown TERM falls back to conservative default",
			vars: map[string]string{"TERM": "some-exotic-terminal"},
			want: Capabilities{ColorLevel: Color16},
		},
		{
			name: "COLORTERM=truecolor upgrades color level",
			vars: map[string]string{"TERM": "xterm", "COLORTERM": "truecolor"},
			want: Capabilities{Name: "xterm", ColorLevel: ColorTrueColor, Mouse: true, BracketedPaste: true, FocusEvents: true},
		},
		{
			name: "iTerm via TERM_PROGRAM",
			vars: map[string]string{"TERM_PROGRAM": "iTerm.app", "TERM": "xterm-256color"},
			want: Capabilities{Name: "iterm", ColorLevel: ColorTrueColor, Mouse: true, BracketedPaste: true, FocusEvents: true, SynchronizedOutput: true},
		},
		{
			name: "empty TERM",
			vars: map[string]string{},
			want: Capabilities{ColorLevel: Color16},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectEnv(env(tt.vars))
			if got != tt.want {
				t.Errorf("DetectEnv(%v) = %+v, want %+v", tt.vars, got, tt.want)
			}
		})
	}
}

func TestProbeRespondsWithinTimeout(t *testing.T) {
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer inR.Close()
	defer inW.Close()

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer outR.Close()
	defer outW.Close()
	go io.Copy(io.Discard, outR)

	if _, err := inW.Write([]byte("\x1b[?1;2c\x1b[?2026;2$y")); err != nil {
		t.Fatal(err)
	}

	got, leftover := Probe(inR, outW, 500*time.Millisecond, Capabilities{ColorLevel: Color16})
	if !got.Detected {
		t.Error("Detected = false, want true (DA1 reply was present)")
	}
	if !got.SynchronizedOutput {
		t.Error("SynchronizedOutput = false, want true (DECRQM reply said Ps=2)")
	}
	if got.ColorLevel != Color16 {
		t.Errorf("ColorLevel = %v, want unchanged base value Color16", got.ColorLevel)
	}
	if len(leftover) != 0 {
		t.Errorf("leftover = %q, want none (input was exactly the two replies)", leftover)
	}
}

// TestProbeReturnsLeftoverBytesNotPartOfReply is a regression test: a
// terminal's reply can arrive alongside bytes that aren't part of it —
// the user's own early keystrokes, or (as actually happened in
// examples/multiplexer) trailing bytes that make it into the same read
// as the reply. Those bytes must come back as leftover, not be
// silently dropped — dropping them is how a terminal's own DA1/DECRQM
// reply ends up forwarded to a child process as if typed by the user.
func TestProbeReturnsLeftoverBytesNotPartOfReply(t *testing.T) {
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer inR.Close()
	defer inW.Close()

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer outR.Close()
	defer outW.Close()
	go io.Copy(io.Discard, outR)

	if _, err := inW.Write([]byte("\x1b[?1;2c\x1b[?2026;2$yhello")); err != nil {
		t.Fatal(err)
	}

	got, leftover := Probe(inR, outW, 500*time.Millisecond, Capabilities{})
	if !got.Detected || !got.SynchronizedOutput {
		t.Fatalf("got = %+v, want both Detected and SynchronizedOutput set", got)
	}
	if string(leftover) != "hello" {
		t.Errorf("leftover = %q, want %q", leftover, "hello")
	}
}

func TestProbeTimesOutWithNoResponse(t *testing.T) {
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer inR.Close()
	defer inW.Close()

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer outR.Close()
	defer outW.Close()
	go io.Copy(io.Discard, outR)

	base := Capabilities{ColorLevel: Color256}
	got, leftover := Probe(inR, outW, 50*time.Millisecond, base)
	if got != base {
		t.Errorf("Probe with no response = %+v, want unchanged base %+v", got, base)
	}
	if len(leftover) != 0 {
		t.Errorf("leftover = %q, want none", leftover)
	}
}

func TestStripLateReply(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"pure DA1 reply", "\x1b[?61;1;21;22c", ""},
		{"pure DECRQM reply", "\x1b[?2026;2$y", ""},
		{"both together", "\x1b[?61;1;21;22c\x1b[?2026;4$y", ""},
		{"reply surrounded by real content", "echo hi\x1b[?1;2crest", "echo hirest"},
		{"no reply present", "just typed text", "just typed text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(StripLateReply([]byte(tt.in)))
			if got != tt.want {
				t.Errorf("StripLateReply(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestProbeStripsUnsupportedDECRQMReply is a regression test for the
// exact reply shape that triggered the original bug: a terminal
// answering "not supported" (Ps=4) for mode 2026 must still be
// recognized and removed from leftover, even though it doesn't set
// SynchronizedOutput. Before the fix, findSyncSupport only recognized
// Ps values that meant "supported" (1 or 2), so a Ps=4 reply was never
// matched at all and leaked through as leftover untouched.
func TestProbeStripsUnsupportedDECRQMReply(t *testing.T) {
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer inR.Close()
	defer inW.Close()

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer outR.Close()
	defer outW.Close()
	go io.Copy(io.Discard, outR)

	if _, err := inW.Write([]byte("\x1b[?61;1;21;22c\x1b[?2026;4$y")); err != nil {
		t.Fatal(err)
	}

	got, leftover := Probe(inR, outW, 500*time.Millisecond, Capabilities{})
	if !got.Detected {
		t.Error("Detected = false, want true (DA1 reply was present)")
	}
	if got.SynchronizedOutput {
		t.Error("SynchronizedOutput = true, want false (Ps=4 means not supported)")
	}
	if len(leftover) != 0 {
		t.Errorf("leftover = %q, want none (both replies should be fully recognized and stripped)", leftover)
	}
}
