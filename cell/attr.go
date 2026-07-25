package cell

// Attr is a bitmask of binary text attributes.
type Attr uint8

const (
	AttrBold Attr = 1 << iota
	AttrFaint
	AttrItalic
	AttrBlink
	AttrReverse
	AttrStrikethrough
	AttrInvisible
)

// UnderlineStyle is the kind of underline a Style applies, if any.
// UnderlineNone (the zero value) means no underline at all — unlike the
// binary Attr flags, underline has more than two states (plain vs.
// curly vs. double, ...), so it gets its own field rather than a bit.
type UnderlineStyle uint8

const (
	UnderlineNone UnderlineStyle = iota
	UnderlineSingle
	UnderlineDouble
	UnderlineCurly
	UnderlineDotted
	UnderlineDashed
)

// Style is a cell's full set of rendering attributes. The zero Style is
// a well-defined, useful default: default foreground on default
// background, no underline, no attributes, no hyperlink.
type Style struct {
	Fg             Color
	Bg             Color
	UnderlineColor Color // meaningful only when Underline != UnderlineNone; DefaultColor() means "same as Fg"
	Underline      UnderlineStyle
	Attr           Attr

	// Hyperlink is the URI of an OSC-8 hyperlink the cell is part of,
	// or "" for none. Added for package vt (M4), which captures OSC 8
	// as exactly this — cell-level metadata — so a Terminal widget can
	// later make the text clickable; see docs/DESIGN.md §7.
	Hyperlink string
}
