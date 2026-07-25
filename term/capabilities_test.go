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

	got := Probe(inR, outW, 500*time.Millisecond, Capabilities{ColorLevel: Color16})
	if !got.Detected {
		t.Error("Detected = false, want true (DA1 reply was present)")
	}
	if !got.SynchronizedOutput {
		t.Error("SynchronizedOutput = false, want true (DECRQM reply said Ps=2)")
	}
	if got.ColorLevel != Color16 {
		t.Errorf("ColorLevel = %v, want unchanged base value Color16", got.ColorLevel)
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
	got := Probe(inR, outW, 50*time.Millisecond, base)
	if got != base {
		t.Errorf("Probe with no response = %+v, want unchanged base %+v", got, base)
	}
}
