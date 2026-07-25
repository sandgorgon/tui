package cell

// ColorKind identifies how a Color's value should be interpreted.
type ColorKind uint8

const (
	// ColorKindDefault means "the terminal's default foreground/background",
	// i.e. no SGR color code should be emitted at all. It's ColorKind's
	// zero value, so the zero Color is a well-defined, useful default.
	ColorKindDefault ColorKind = iota
	// ColorKindANSI is one of the 16 basic ANSI colors (0-15, including
	// the 8 bright variants), stored in Color.R.
	ColorKindANSI
	// ColorKindIndexed is a 256-color palette index (0-255), stored in
	// Color.R.
	ColorKindIndexed
	// ColorKindRGB is a 24-bit truecolor value in Color.R/G/B.
	ColorKindRGB
)

// Color is a terminal color. It's a plain 4-byte value (comparable with
// ==), deliberately not carrying any notion of how to downsample a
// truecolor value for a terminal that can't display it — that's an
// output-encoding concern belonging to package render, not this
// data-model layer (see docs/DESIGN.md §4).
type Color struct {
	Kind    ColorKind
	R, G, B uint8
}

// DefaultColor is the terminal's default foreground/background — the
// zero Color. Provided for readability at call sites.
func DefaultColor() Color { return Color{} }

// ANSIColor returns one of the 16 basic ANSI colors. i should be 0-15;
// values outside that range are stored as-is (callers doing their own
// palette math may rely on this), but won't correspond to a real ANSI
// color.
func ANSIColor(i uint8) Color { return Color{Kind: ColorKindANSI, R: i} }

// IndexedColor returns a 256-color palette entry.
func IndexedColor(i uint8) Color { return Color{Kind: ColorKindIndexed, R: i} }

// RGBColor returns a 24-bit truecolor value.
func RGBColor(r, g, b uint8) Color { return Color{Kind: ColorKindRGB, R: r, G: g, B: b} }
