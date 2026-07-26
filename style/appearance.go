package style

import "strconv"

// Appearance is whether a Theme is tuned for a light or dark terminal
// background.
type Appearance int

const (
	Dark Appearance = iota
	Light
)

func (a Appearance) String() string {
	if a == Light {
		return "Light"
	}
	return "Dark"
}

// DetectAppearance derives a best-effort guess of the terminal's
// background from the $COLORFGBG environment variable ("fg;bg", set
// by many terminals — e.g. xterm and most terminals that emulate it —
// to the ANSI color indices of their default foreground/background),
// falling back to Dark (the common case, and a safe default: a Dark
// theme's accent colors stay reasonably legible even on an
// undetected light background, which isn't true the other way
// around) when $COLORFGBG is absent or unparseable.
//
// This is the same layered, no-terminfo-database philosophy as
// term.DetectEnv (docs/DESIGN.md §3.4): a cheap environment-variable
// heuristic, not a real query — there's no dependable, universal
// escape sequence for "what's your background color" the way there is
// for DA1/DA2 device attributes. Production callers should pass
// os.Getenv; tests can pass a fake.
func DetectAppearance(getenv func(string) string) Appearance {
	fgbg := getenv("COLORFGBG")
	i := len(fgbg) - 1
	for i >= 0 && fgbg[i] != ';' {
		i--
	}
	if i < 0 {
		return Dark
	}
	bg, err := strconv.Atoi(fgbg[i+1:])
	if err != nil {
		return Dark
	}
	// COLORFGBG's background is a basic ANSI index (0-15); 7 (white)
	// and 15 (bright white) are the light-background terminals in
	// practice, everything else reads as dark.
	if bg == 7 || bg == 15 {
		return Light
	}
	return Dark
}
