package style

import "github.com/sandgorgon/tui/cell"

// Theme is a small, named set of semantic colors: a widget picks a
// color by role (theme.Primary, theme.Error, ...) instead of a raw
// cell.Color, so swapping the Theme in use — light for dark, or a
// user's own palette — restyles every widget built against it without
// touching widget code. It's a layer on top of cell.Color/cell.Style,
// not a redefinition of them (docs/DESIGN.md §4 — Color/Style/Attr
// live in package cell, resolved at M2).
//
// Theme is a plain struct, not an opaque type: every field can be
// overridden individually, e.g. `t := style.DefaultDark(); t.Error =
// cell.RGBColor(255,0,0)`.
type Theme struct {
	Appearance Appearance

	// Foreground and Background are the terminal's default text/window
	// colors — the zero cell.Color (see cell.DefaultColor), on purpose:
	// a well-behaved TUI leaves the user's own terminal color scheme
	// alone for plain text, and only asserts explicit color for the
	// semantic roles below, which need to stay legible regardless of
	// what that scheme is.
	Foreground cell.Color
	Background cell.Color

	Primary   cell.Color // the theme's main accent, e.g. selected/active elements
	Secondary cell.Color
	Accent    cell.Color

	// Muted is dimmed text: placeholders, disabled state, help text.
	// Its contrast is only guaranteed against the terminal's own
	// background (see DefaultDark/DefaultLight) — it is not guaranteed
	// readable painted on top of Border. Use ChromeText for that.
	Muted cell.Color

	// Border is the default (unfocused) border/divider color. It also
	// doubles as the background for a tinted "chrome" panel (e.g. a
	// status bar or gutter) via BorderStyle() — pair it with ChromeText,
	// not Muted, when painting text on top of it; see ChromeText's doc
	// comment.
	Border cell.Color

	Focus cell.Color // focused-element border/indicator color

	// Chrome is text painted on a Border-tinted background (a status
	// bar, gutter, or other chrome panel using BorderStyle() as its
	// background) — see ChromeText(). Its contrast is guaranteed
	// against Border, not against the terminal's own background.
	Chrome cell.Color

	Success cell.Color
	Warning cell.Color
	Error   cell.Color
	Info    cell.Color
}

// DefaultDark is a sensible default Theme for a dark terminal
// background.
//
// Every role here is chosen to survive three degradations a plain
// truecolor pick doesn't account for, checked against a representative
// dark background (#1e1e1e): WCAG contrast (Border >=3:1, the non-text/
// UI-component minimum; everything else >=4.5:1), a red-green
// colorblindness simulation (Success/Warning/Error stay well separated
// under both protanopia and deuteranopia, not just in raw hue), and
// render.rgbToIndexed16's nearest-ANSI-16 downsampling (used for the
// Color16 terminal tier declared in package term) landing each of
// Success/Warning/Error on its own distinct slot instead of collapsing
// together. ChromeText additionally holds >=4.5:1 against Border itself
// (not just the terminal background), since it's meant to be painted on
// top of a Border-tinted chrome panel — see style/theme_test.go for the
// regression checks.
func DefaultDark() Theme {
	return Theme{
		Appearance: Dark,
		Primary:    cell.RGBColor(97, 175, 239),
		Secondary:  cell.RGBColor(198, 120, 221),
		Accent:     cell.RGBColor(86, 182, 194),
		Muted:      cell.RGBColor(145, 151, 163),
		Border:     cell.RGBColor(102, 110, 124),
		Focus:      cell.RGBColor(97, 175, 239),
		Chrome:     cell.RGBColor(242, 244, 247),
		Success:    cell.RGBColor(35, 212, 85),
		Warning:    cell.RGBColor(255, 220, 4),
		Error:      cell.RGBColor(225, 95, 30),
		Info:       cell.RGBColor(86, 182, 194),
	}
}

// DefaultLight is a sensible default Theme for a light terminal
// background.
//
// Tuned against a representative light background (#f5f5f5) under the
// same constraints as DefaultDark: WCAG contrast (including ChromeText
// against Border itself), colorblindness separation for
// Success/Warning/Error, and distinct ANSI-16 slots for that same trio.
// See DefaultDark's doc comment and style/theme_test.go.
func DefaultLight() Theme {
	return Theme{
		Appearance: Light,
		Primary:    cell.RGBColor(33, 110, 182),
		Secondary:  cell.RGBColor(136, 54, 157),
		Accent:     cell.RGBColor(19, 124, 134),
		Muted:      cell.RGBColor(118, 118, 122),
		Border:     cell.RGBColor(136, 136, 144),
		Focus:      cell.RGBColor(33, 110, 182),
		Chrome:     cell.RGBColor(20, 20, 23),
		Success:    cell.RGBColor(3, 138, 94),
		Warning:    cell.RGBColor(150, 111, 18),
		Error:      cell.RGBColor(148, 9, 31),
		Info:       cell.RGBColor(19, 124, 134),
	}
}

// Default returns DefaultDark or DefaultLight according to appearance.
func Default(appearance Appearance) Theme {
	if appearance == Light {
		return DefaultLight()
	}
	return DefaultDark()
}

// Text returns a plain cell.Style using the theme's foreground and
// background (the terminal's own defaults, per the Theme.Foreground
// doc comment).
func (t Theme) Text() cell.Style {
	return cell.Style{Fg: t.Foreground, Bg: t.Background}
}

// MutedText returns a dimmed cell.Style, e.g. for placeholder or
// disabled text, painted on the terminal's own background. Its
// contrast is not guaranteed against a Border-tinted background — use
// ChromeText for text painted on top of BorderStyle().
func (t Theme) MutedText() cell.Style {
	return cell.Style{Fg: t.Muted, Bg: t.Background}
}

// BorderStyle returns the style for an unfocused border or divider.
// Its Fg (Border) doubles as a background a caller can reuse to tint a
// chrome panel, e.g. cell.Style{Fg: theme.ChromeText, Bg: theme.Border}
// (or ChromeText()) for a status bar or gutter that should read as
// distinct from body content.
func (t Theme) BorderStyle() cell.Style {
	return cell.Style{Fg: t.Border, Bg: t.Background}
}

// ChromeText returns the style for text painted on a Border-tinted
// chrome panel (a status bar, gutter, or similar) — the background
// callers get from BorderStyle().Fg. Unlike MutedText, its contrast is
// guaranteed against Border, not the terminal's own background.
func (t Theme) ChromeText() cell.Style {
	return cell.Style{Fg: t.Chrome, Bg: t.Border}
}

// FocusStyle returns the style for a focused border or indicator, e.g.
// what a tui.Widget draws in Paint when SetFocused(true) was called.
func (t Theme) FocusStyle() cell.Style {
	return cell.Style{Fg: t.Focus, Bg: t.Background, Attr: cell.AttrBold}
}
