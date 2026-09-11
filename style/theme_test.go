package style

import (
	"math"
	"testing"

	"github.com/sandgorgon/tui/cell"
)

func TestDefaultRespectsAppearance(t *testing.T) {
	if got := Default(Dark).Appearance; got != Dark {
		t.Errorf("Default(Dark).Appearance = %v, want Dark", got)
	}
	if got := Default(Light).Appearance; got != Light {
		t.Errorf("Default(Light).Appearance = %v, want Light", got)
	}
}

func TestDefaultThemesLeaveForegroundBackgroundAtTerminalDefault(t *testing.T) {
	for _, th := range []Theme{DefaultDark(), DefaultLight()} {
		if th.Foreground != cell.DefaultColor() {
			t.Errorf("%v theme Foreground = %+v, want the zero/default color", th.Appearance, th.Foreground)
		}
		if th.Background != cell.DefaultColor() {
			t.Errorf("%v theme Background = %+v, want the zero/default color", th.Appearance, th.Background)
		}
	}
}

func TestDefaultThemesSetEverySemanticRole(t *testing.T) {
	zero := cell.Color{}
	for _, th := range []Theme{DefaultDark(), DefaultLight()} {
		roles := map[string]cell.Color{
			"Primary": th.Primary, "Secondary": th.Secondary, "Accent": th.Accent,
			"Muted": th.Muted, "Border": th.Border, "Focus": th.Focus, "Chrome": th.Chrome,
			"Success": th.Success, "Warning": th.Warning, "Error": th.Error, "Info": th.Info,
		}
		for name, c := range roles {
			if c == zero {
				t.Errorf("%v theme's %s role is the zero Color (unset)", th.Appearance, name)
			}
		}
	}
}

// representativeBg is a stand-in for "a real terminal's background",
// used only to compute contrast ratios in these tests. It's not the
// terminal's actual background (Theme deliberately never asserts
// one — see Theme.Foreground's doc comment); it's a fixed reference
// point (matching the values cited in DefaultDark/DefaultLight's doc
// comments) so the tests are deterministic instead of guessing at
// whatever background the CI terminal happens to have.
func representativeBg(a Appearance) (r, g, b uint8) {
	if a == Light {
		return 245, 245, 245
	}
	return 30, 30, 30
}

// wcagContrast computes the WCAG 2.1 contrast ratio between two sRGB
// colors (1:1 to 21:1).
func wcagContrast(r1, g1, b1, r2, g2, b2 uint8) float64 {
	l1, l2 := relativeLuminance(r1, g1, b1), relativeLuminance(r2, g2, b2)
	hi, lo := l1, l2
	if lo > hi {
		hi, lo = lo, hi
	}
	return (hi + 0.05) / (lo + 0.05)
}

func relativeLuminance(r, g, b uint8) float64 {
	lin := func(c uint8) float64 {
		v := float64(c) / 255
		if v <= 0.03928 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

// TestDefaultThemesMeetContrastMinimums guards against colors that are
// too close to the background to read: Border only needs to clear
// WCAG's 3:1 non-text/UI-component minimum (it's a divider, not
// text), everything else that stands in for text or an icon needs the
// 4.0:1 floor documented on DefaultDark/DefaultLight (deliberately
// just under the 4.5:1 AA body-text threshold, since Muted in
// particular is meant to visually recede, not read like body text).
// Chrome is checked here against the representative terminal
// background too (it's occasionally composed straight over it, e.g.
// via Text()-style usage), but its load-bearing guarantee is against
// Border itself — see TestChromeTextReadableOnBorder.
func TestDefaultThemesMeetContrastMinimums(t *testing.T) {
	for _, th := range []Theme{DefaultDark(), DefaultLight()} {
		bgR, bgG, bgB := representativeBg(th.Appearance)
		check := func(name string, c cell.Color, min float64) {
			t.Helper()
			got := wcagContrast(c.R, c.G, c.B, bgR, bgG, bgB)
			if got < min {
				t.Errorf("%v theme's %s = %v, contrast against representative bg = %.2f, want >= %.1f", th.Appearance, name, c, got, min)
			}
		}
		check("Primary", th.Primary, 4.0)
		check("Secondary", th.Secondary, 4.0)
		check("Accent", th.Accent, 4.0)
		check("Muted", th.Muted, 4.0)
		check("Border", th.Border, 3.0)
		check("Chrome", th.Chrome, 4.0)
		check("Success", th.Success, 4.0)
		check("Warning", th.Warning, 4.0)
		check("Error", th.Error, 4.0)
		check("Info", th.Info, 4.0)
	}
}

// TestChromeTextReadableOnBorder guards against the gap that let
// MutedText-on-Border ship at ~1.2-1.5:1 contrast (near-unreadable —
// see the issue that added Chrome/ChromeText): unlike every other
// role, Border is routinely used as a *background* (BorderStyle()'s
// Fg, reused as Bg for a tinted chrome panel — a status bar or
// gutter), so whatever text goes on top of it needs its contrast
// checked against Border itself, not just against the terminal
// background. Chrome is the role meant for exactly that composition;
// it's held to the real 4.5:1 AA body-text minimum here (not the
// relaxed 4.0:1 floor used elsewhere for roles like Muted that are
// deliberately allowed to recede) since it has no other job.
func TestChromeTextReadableOnBorder(t *testing.T) {
	for _, th := range []Theme{DefaultDark(), DefaultLight()} {
		got := wcagContrast(th.Chrome.R, th.Chrome.G, th.Chrome.B, th.Border.R, th.Border.G, th.Border.B)
		if got < 4.5 {
			t.Errorf("%v theme: Chrome %v on Border %v = %.2f contrast, want >= 4.5", th.Appearance, th.Chrome, th.Border, got)
		}
	}
}

// dichromacySimulate applies a Machado et al. (2009) severity-1.0
// dichromacy simulation matrix (in linear RGB) to an sRGB color,
// returning the sRGB color a protanope/deuteranope would see in its
// place.
func dichromacySimulate(r, g, b uint8, m [3][3]float64) (uint8, uint8, uint8) {
	lin := func(c uint8) float64 {
		v := float64(c) / 255
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	unlin := func(v float64) uint8 {
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		var s float64
		if v <= 0.0031308 {
			s = 12.92 * v
		} else {
			s = 1.055*math.Pow(v, 1/2.4) - 0.055
		}
		return uint8(math.Round(255 * s))
	}
	lr, lg, lb := lin(r), lin(g), lin(b)
	out := [3]float64{}
	in := [3]float64{lr, lg, lb}
	for i := range 3 {
		for j := range 3 {
			out[i] += m[i][j] * in[j]
		}
	}
	return unlin(out[0]), unlin(out[1]), unlin(out[2])
}

var (
	protanopiaMatrix = [3][3]float64{
		{0.152286, 1.052583, -0.204868},
		{0.114503, 0.786281, 0.099216},
		{-0.003882, -0.048116, 1.051998},
	}
	deuteranopiaMatrix = [3][3]float64{
		{0.367322, 0.860646, -0.227968},
		{0.280085, 0.672501, 0.047413},
		{-0.011820, 0.042940, 0.968881},
	}
)

func rgbDist(r1, g1, b1, r2, g2, b2 uint8) float64 {
	dr := float64(r1) - float64(r2)
	dg := float64(g1) - float64(g2)
	db := float64(b1) - float64(b2)
	return math.Sqrt(dr*dr + dg*dg + db*db)
}

// TestAlertTriadSurvivesColorblindness guards against Success/Warning/
// Error becoming indistinguishable under red-green colorblindness
// (protanopia/deuteranopia, the common forms, ~5-8% of men): raw hue
// separation between these three roles isn't enough on its own, since
// dichromacy simulation can collapse colors that look very different
// in normal vision (this caught DefaultDark's original Success/
// Warning, which were 77 apart in raw RGB but ~9 apart simulated).
// minSeparation is well under the ~59-149 the current palette
// actually achieves (see DefaultDark/DefaultLight's doc comments),
// leaving room for future palette tweaks without being a tripwire.
func TestAlertTriadSurvivesColorblindness(t *testing.T) {
	const minSeparation = 35

	for _, th := range []Theme{DefaultDark(), DefaultLight()} {
		roles := map[string]cell.Color{"Success": th.Success, "Warning": th.Warning, "Error": th.Error}
		names := []string{"Success", "Warning", "Error"}
		for _, matrix := range []struct {
			name string
			m    [3][3]float64
		}{{"protanopia", protanopiaMatrix}, {"deuteranopia", deuteranopiaMatrix}} {
			for i := range len(names) {
				for j := i + 1; j < len(names); j++ {
					a, b := roles[names[i]], roles[names[j]]
					ar, ag, ab := dichromacySimulate(a.R, a.G, a.B, matrix.m)
					br, bg, bb := dichromacySimulate(b.R, b.G, b.B, matrix.m)
					dist := rgbDist(ar, ag, ab, br, bg, bb)
					if dist < minSeparation {
						t.Errorf("%v theme: %s vs %s separation under %s = %.1f, want >= %v (colors %v/%v become hard to tell apart)",
							th.Appearance, names[i], names[j], matrix.name, dist, minSeparation, a, b)
					}
				}
			}
		}
	}
}

// ansi16Palette mirrors render.ansi16Palette (xterm's conventional RGB
// values for the 16 basic ANSI colors) so this test can predict which
// slot render's nearest-neighbor downsampler (render.rgbToIndexed16)
// will pick for a given truecolor value, without style importing
// render or render exporting internals purely for this test.
var ansi16Palette = [16][3]uint8{
	{0, 0, 0}, {205, 0, 0}, {0, 205, 0}, {205, 205, 0},
	{0, 0, 238}, {205, 0, 205}, {0, 205, 205}, {229, 229, 229},
	{127, 127, 127}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0},
	{92, 92, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
}

func nearestANSI16(c cell.Color) int {
	best, bestDist := 0, math.MaxFloat64
	for i, p := range ansi16Palette {
		d := rgbDist(c.R, c.G, c.B, p[0], p[1], p[2])
		if d < bestDist {
			bestDist, best = d, i
		}
	}
	return best
}

// TestAlertTriadDistinctOn16ColorTerminals guards against Success/
// Warning/Error collapsing onto the same basic ANSI color once
// downsampled for the Color16 terminal tier (term.Capabilities —
// a real, targeted fallback: plain "xterm" TERM and the Linux console
// both report it). This caught DefaultDark's original palette, where
// Muted/Border/Success/Error all downsampled to the same bright-black
// gray — Success and Error, meant to be opposite-valence signals,
// rendered identically.
func TestAlertTriadDistinctOn16ColorTerminals(t *testing.T) {
	for _, th := range []Theme{DefaultDark(), DefaultLight()} {
		slots := map[int][]string{}
		for _, role := range []struct {
			name string
			c    cell.Color
		}{{"Success", th.Success}, {"Warning", th.Warning}, {"Error", th.Error}} {
			slots[nearestANSI16(role.c)] = append(slots[nearestANSI16(role.c)], role.name)
		}
		for slot, names := range slots {
			if len(names) > 1 {
				t.Errorf("%v theme: %v all downsample to the same ANSI-16 color (slot %d) on a Color16 terminal", th.Appearance, names, slot)
			}
		}
	}
}

// TestAccentAndInfoAreDistinct guards against the regression where
// DefaultDark/DefaultLight gave Accent and Info the identical RGB
// value (github.com/sandgorgon/tui#40): a consumer picking theme
// roles by reading Theme's field list has no way to discover a
// collision like that short of diffing the raw RGB values, so it's
// checked here the same way the Success/Warning/Error triad is —
// raw-RGB distinctness plus distinct ANSI-16 slots, so the two roles
// don't collapse together on a Color16 terminal either.
func TestAccentAndInfoAreDistinct(t *testing.T) {
	for _, th := range []Theme{DefaultDark(), DefaultLight()} {
		if th.Accent == th.Info {
			t.Errorf("%v theme: Accent and Info are the identical color %v", th.Appearance, th.Accent)
		}
		const minSeparation = 35
		if d := rgbDist(th.Accent.R, th.Accent.G, th.Accent.B, th.Info.R, th.Info.G, th.Info.B); d < minSeparation {
			t.Errorf("%v theme: Accent %v vs Info %v separation = %.1f, want >= %v", th.Appearance, th.Accent, th.Info, d, minSeparation)
		}
		if as, is := nearestANSI16(th.Accent), nearestANSI16(th.Info); as == is {
			t.Errorf("%v theme: Accent %v and Info %v both downsample to the same ANSI-16 color (slot %d) on a Color16 terminal", th.Appearance, th.Accent, th.Info, as)
		}
	}
}

func TestThemeStyleHelpersUseExpectedRoles(t *testing.T) {
	th := DefaultDark()

	if got := th.Text(); got.Fg != th.Foreground || got.Bg != th.Background {
		t.Errorf("Text() = %+v", got)
	}
	if got := th.MutedText(); got.Fg != th.Muted {
		t.Errorf("MutedText().Fg = %+v, want theme.Muted", got.Fg)
	}
	if got := th.BorderStyle(); got.Fg != th.Border {
		t.Errorf("BorderStyle().Fg = %+v, want theme.Border", got.Fg)
	}
	if got := th.ChromeText(); got.Fg != th.Chrome || got.Bg != th.Border {
		t.Errorf("ChromeText() = %+v, want Fg=theme.Chrome, Bg=theme.Border", got)
	}
	focus := th.FocusStyle()
	if focus.Fg != th.Focus {
		t.Errorf("FocusStyle().Fg = %+v, want theme.Focus", focus.Fg)
	}
	if focus.Attr&cell.AttrBold == 0 {
		t.Error("FocusStyle() should be bold to stand out from BorderStyle()")
	}
}
