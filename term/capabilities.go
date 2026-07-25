package term

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"time"
)

// ColorLevel is a terminal's supported color depth.
type ColorLevel int

const (
	ColorNone ColorLevel = iota
	Color16
	Color256
	ColorTrueColor
)

// Capabilities describes what a terminal is believed to support. See
// docs/DESIGN.md §3.4: detection is layered — environment variables and
// a small built-in table first (DetectEnv), optionally strengthened by
// an active escape-sequence probe (Probe) — with no dependency on a
// system terminfo/termcap database.
type Capabilities struct {
	Name               string
	ColorLevel         ColorLevel
	Mouse              bool
	BracketedPaste     bool
	FocusEvents        bool
	SynchronizedOutput bool
	KittyKeyboard      bool

	// Detected is set by Probe when the terminal actually answered an
	// active query, i.e. we know we're really talking to a live
	// terminal rather than guessing from environment variables alone.
	Detected bool
}

// builtinTable is a small set of capability baselines for terminals
// that matter in practice, keyed by a normalized $TERM prefix.
var builtinTable = map[string]Capabilities{
	"xterm-256color": {ColorLevel: ColorTrueColor, Mouse: true, BracketedPaste: true, FocusEvents: true},
	"xterm":          {ColorLevel: Color16, Mouse: true, BracketedPaste: true, FocusEvents: true},
	"screen":         {ColorLevel: Color256, Mouse: true, BracketedPaste: true},
	"tmux":           {ColorLevel: ColorTrueColor, Mouse: true, BracketedPaste: true, FocusEvents: true, SynchronizedOutput: true},
	"alacritty":      {ColorLevel: ColorTrueColor, Mouse: true, BracketedPaste: true, FocusEvents: true, SynchronizedOutput: true},
	"foot":           {ColorLevel: ColorTrueColor, Mouse: true, BracketedPaste: true, FocusEvents: true, SynchronizedOutput: true},
	"wezterm":        {ColorLevel: ColorTrueColor, Mouse: true, BracketedPaste: true, FocusEvents: true, SynchronizedOutput: true, KittyKeyboard: true},
	"kitty":          {ColorLevel: ColorTrueColor, Mouse: true, BracketedPaste: true, FocusEvents: true, SynchronizedOutput: true, KittyKeyboard: true},
	"rxvt":           {ColorLevel: Color256, Mouse: true},
	"linux":          {ColorLevel: Color16},
	"vt100":          {ColorLevel: ColorNone},
}

// termPrefixes is the order in which $TERM is matched against
// builtinTable when there's no exact match (e.g. "xterm-kitty",
// "tmux-256color").
var termPrefixes = []string{
	"tmux", "screen", "xterm", "rxvt", "alacritty", "kitty", "foot", "wezterm", "linux", "vt100",
}

// DetectEnv derives a baseline Capabilities purely from environment
// variables and the built-in table — no I/O with the terminal itself,
// so it's safe to call even when output isn't connected to a tty.
// Production callers should pass os.Getenv; tests can pass a fake.
func DetectEnv(getenv func(string) string) Capabilities {
	base := Capabilities{ColorLevel: Color16}

	switch getenv("TERM_PROGRAM") {
	case "iTerm.app":
		base = Capabilities{Name: "iterm", ColorLevel: ColorTrueColor, Mouse: true, BracketedPaste: true, FocusEvents: true, SynchronizedOutput: true}
	case "WezTerm":
		base = builtinTable["wezterm"]
		base.Name = "wezterm"
	case "vscode":
		base = Capabilities{Name: "vscode", ColorLevel: ColorTrueColor, Mouse: true, BracketedPaste: true, FocusEvents: true}
	case "Apple_Terminal":
		base = Capabilities{Name: "apple-terminal", ColorLevel: Color256, Mouse: true, BracketedPaste: true}
	default:
		if c, ok := lookupTerm(getenv("TERM")); ok {
			base = c
		}
	}

	switch getenv("COLORTERM") {
	case "truecolor", "24bit":
		base.ColorLevel = ColorTrueColor
	}

	return base
}

func lookupTerm(term string) (Capabilities, bool) {
	if term == "" {
		return Capabilities{}, false
	}
	if c, ok := builtinTable[term]; ok {
		c.Name = term
		return c, true
	}
	for _, prefix := range termPrefixes {
		if strings.HasPrefix(term, prefix) {
			if c, ok := builtinTable[prefix]; ok {
				c.Name = term
				return c, true
			}
		}
	}
	return Capabilities{}, false
}

// Probe strengthens base with information from an active escape-sequence
// query: a DA1 device-attributes request (to confirm something is really
// listening) and a DECRQM query for DEC synchronized-output mode 2026.
// If in/out aren't connected to a real, responsive terminal, or nothing
// arrives within timeout, base is returned unchanged.
//
// Any bytes Probe reads that aren't part of a recognized reply are
// returned as leftover — bytes that arrive during the probe window
// aren't necessarily the terminal's reply; they could be the user's own
// early keystrokes, and either way they must not be silently discarded.
// Callers MUST prepend leftover to whatever they read next (feed it to
// their input.Decoder, or forward it to a pty) rather than drop it. This
// isn't optional: a terminal's reply can legitimately arrive after
// Probe's timeout (a slow terminal, or one that batches replies), and if
// Probe simply discarded unread-but-arriving-late bytes, they'd still
// land in the file descriptor and get picked up by whatever reads it
// next — which, in a program that forwards raw input to a child (e.g. a
// pty), means the terminal's own DA1/DECRQM reply ends up forwarded as
// if the user had typed it. That's not hypothetical: it's exactly what
// happened before this was fixed (see examples/multiplexer).
func Probe(in, out *os.File, timeout time.Duration, base Capabilities) (Capabilities, []byte) {
	if _, err := out.Write([]byte("\x1b[c\x1b[?2026$p")); err != nil {
		return base, nil
	}

	// If nothing answers at all (piped output, dumb terminal), wait the
	// full timeout. But once a reply starts arriving, only wait a short
	// quiet period for any trailing bytes rather than the full budget —
	// a responsive terminal shouldn't make every startup pay the whole
	// timeout cost.
	const quiet = 20 * time.Millisecond
	deadline := time.Now().Add(timeout)

	var total []byte
	buf := make([]byte, 256)
	for len(total) < 256 {
		if err := in.SetReadDeadline(deadline); err != nil {
			return base, total
		}
		n, err := in.Read(buf)
		if n > 0 {
			total = append(total, buf[:n]...)
			if d := time.Now().Add(quiet); d.Before(deadline) {
				deadline = d
			}
		}
		if err != nil {
			break
		}
	}
	in.SetReadDeadline(time.Time{})

	result := base
	remaining := total
	if start, end, ok := findDA1(remaining); ok {
		result.Detected = true
		remaining = cutRange(remaining, start, end)
	}
	if start, end, ps, ok := findDECRQM2026(remaining); ok {
		// Ps 1/2 (currently set/reset, togglable) and 3 (permanently
		// set — always on, can't be turned off, but still supported)
		// all mean the terminal recognizes the mode. Only 0 (not
		// recognized) and 4 (permanently reset — never supported) mean
		// it doesn't.
		if ps == 1 || ps == 2 || ps == 3 {
			result.SynchronizedOutput = true
		}
		remaining = cutRange(remaining, start, end)
	}
	if len(remaining) == 0 {
		return result, nil
	}
	return result, remaining
}

// StripLateReply removes any DA1/DECRQM reply from b and returns the
// result. Probe can only rescue reply bytes it reads *within its own
// timeout window* (see its leftover return); a reply that arrives
// after Probe has already given up — a slow terminal, or one that
// batches escape-sequence replies — is invisible to Probe entirely, and
// would otherwise be read (and, in a program that forwards raw input to
// a child, forwarded) by whatever reads the terminal next, looking
// exactly like typed input.
//
// A caller that persistently owns stdin after calling Probe (a pty
// multiplexer, an input decoder loop) should pass every read through
// StripLateReply for a short grace period after startup (a couple of
// seconds is plenty — by then the terminal has either replied or isn't
// going to). The patterns matched are specific enough (they must start
// with ESC, then "[?", then digits/semicolons in an exact shape) that a
// human typing or pasting real input matching one by coincidence is
// not a realistic concern.
func StripLateReply(b []byte) []byte {
	for {
		matched := false
		if start, end, ok := findDA1(b); ok {
			b = cutRange(b, start, end)
			matched = true
		}
		if start, end, _, ok := findDECRQM2026(b); ok {
			b = cutRange(b, start, end)
			matched = true
		}
		if !matched {
			return b
		}
	}
}

// cutRange returns b with the byte range [start,end) removed.
func cutRange(b []byte, start, end int) []byte {
	out := make([]byte, 0, len(b)-(end-start))
	out = append(out, b[:start]...)
	return append(out, b[end:]...)
}

// findDA1 reports the byte range of the first DA1 reply in b: CSI ? ... c.
func findDA1(b []byte) (start, end int, ok bool) {
	i := indexCSI(b, 0)
	for i >= 0 {
		e := i + 2
		for e < len(b) && (b[e] == '?' || b[e] == ';' || (b[e] >= '0' && b[e] <= '9')) {
			e++
		}
		if e < len(b) && b[e] == 'c' {
			return i, e + 1, true
		}
		i = indexCSI(b, i+1)
	}
	return 0, 0, false
}

// findDECRQM2026 reports the byte range of any DECRQM reply for mode
// 2026 — CSI ? 2026 ; Ps $ y — and its Ps value, regardless of what Ps
// says. This is deliberately more permissive than "does Ps indicate
// support": Probe uses the Ps value to decide SynchronizedOutput, but
// both Probe and StripLateReply need to recognize (and remove) the
// reply from the stream regardless of its answer — a reply saying
// "not supported" (Ps 0, 3, or 4) is still our own query's reply, and
// leaving it unmatched here means leaving it unremoved, which is
// exactly the bug this exists to avoid.
func findDECRQM2026(b []byte) (start, end, ps int, ok bool) {
	const prefix = "\x1b[?2026;"
	i := bytes.Index(b, []byte(prefix))
	if i < 0 {
		return 0, 0, 0, false
	}
	rest := b[i+len(prefix):]
	j := bytes.Index(rest, []byte("$y"))
	if j < 0 {
		return 0, 0, 0, false
	}
	psVal, err := strconv.Atoi(string(rest[:j]))
	if err != nil {
		return 0, 0, 0, false
	}
	return i, i + len(prefix) + j + 2, psVal, true
}

// indexCSI returns the index of the next "\x1b[" in b at or after from,
// or -1.
func indexCSI(b []byte, from int) int {
	for i := from; i+1 < len(b); i++ {
		if b[i] == 0x1b && b[i+1] == '[' {
			return i
		}
	}
	return -1
}
