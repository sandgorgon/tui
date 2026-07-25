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
// background, no underline, no attributes.
type Style struct {
	Fg             Color
	Bg             Color
	UnderlineColor Color // meaningful only when Underline != UnderlineNone; DefaultColor() means "same as Fg"
	Underline      UnderlineStyle
	Attr           Attr
}
