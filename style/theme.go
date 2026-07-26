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

	Muted  cell.Color // dimmed text: placeholders, disabled state, help text
	Border cell.Color // default (unfocused) border/divider color
	Focus  cell.Color // focused-element border/indicator color

	Success cell.Color
	Warning cell.Color
	Error   cell.Color
	Info    cell.Color
}

// DefaultDark is a sensible default Theme for a dark terminal
// background.
func DefaultDark() Theme {
	return Theme{
		Appearance: Dark,
		Primary:    cell.RGBColor(97, 175, 239),
		Secondary:  cell.RGBColor(198, 120, 221),
		Accent:     cell.RGBColor(86, 182, 194),
		Muted:      cell.RGBColor(92, 99, 112),
		Border:     cell.RGBColor(60, 66, 78),
		Focus:      cell.RGBColor(97, 175, 239),
		Success:    cell.RGBColor(152, 195, 121),
		Warning:    cell.RGBColor(229, 192, 123),
		Error:      cell.RGBColor(224, 108, 117),
		Info:       cell.RGBColor(86, 182, 194),
	}
}

// DefaultLight is a sensible default Theme for a light terminal
// background.
func DefaultLight() Theme {
	return Theme{
		Appearance: Light,
		Primary:    cell.RGBColor(33, 110, 182),
		Secondary:  cell.RGBColor(136, 54, 157),
		Accent:     cell.RGBColor(19, 124, 134),
		Muted:      cell.RGBColor(140, 140, 140),
		Border:     cell.RGBColor(200, 200, 205),
		Focus:      cell.RGBColor(33, 110, 182),
		Success:    cell.RGBColor(58, 130, 45),
		Warning:    cell.RGBColor(163, 110, 0),
		Error:      cell.RGBColor(179, 45, 50),
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
// disabled text.
func (t Theme) MutedText() cell.Style {
	return cell.Style{Fg: t.Muted, Bg: t.Background}
}

// BorderStyle returns the style for an unfocused border or divider.
func (t Theme) BorderStyle() cell.Style {
	return cell.Style{Fg: t.Border, Bg: t.Background}
}

// FocusStyle returns the style for a focused border or indicator, e.g.
// what a tui.Widget draws in Paint when SetFocused(true) was called.
func (t Theme) FocusStyle() cell.Style {
	return cell.Style{Fg: t.Focus, Bg: t.Background, Attr: cell.AttrBold}
}
