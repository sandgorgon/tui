package term

import (
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
// Probe consumes whatever bytes arrive on in during the probe window, so
// it must run before the application starts reading real input — any
// keystroke the user makes during that (short) window would otherwise be
// lost. This is the standard, universally-accepted tradeoff of active
// terminal probing at startup.
func Probe(in, out *os.File, timeout time.Duration, base Capabilities) Capabilities {
	if _, err := out.Write([]byte("\x1b[c\x1b[?2026$p")); err != nil {
		return base
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
			return base
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
	if hasDA1(total) {
		result.Detected = true
	}
	if hasSyncSupport(total) {
		result.SynchronizedOutput = true
	}
	return result
}

// hasDA1 reports whether b contains a DA1 reply: CSI ? ... c.
func hasDA1(b []byte) bool {
	i := indexCSI(b, 0)
	for i >= 0 {
		end := i + 2
		for end < len(b) && b[end] != 'c' && (b[end] == '?' || b[end] == ';' || (b[end] >= '0' && b[end] <= '9')) {
			end++
		}
		if end < len(b) && b[end] == 'c' {
			return true
		}
		i = indexCSI(b, i+1)
	}
	return false
}

// hasSyncSupport reports whether b contains a DECRQM reply for mode
// 2026 indicating support: CSI ? 2026 ; Ps $ y, with Ps == 1 or 2.
func hasSyncSupport(b []byte) bool {
	s := string(b)
	const prefix = "\x1b[?2026;"
	_, rest, ok := strings.Cut(s, prefix)
	if !ok {
		return false
	}
	psStr, _, ok := strings.Cut(rest, "$y")
	if !ok {
		return false
	}
	ps, err := strconv.Atoi(psStr)
	return err == nil && (ps == 1 || ps == 2)
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
