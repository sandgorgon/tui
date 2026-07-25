package layout

type constraintKind uint8

const (
	kindLength constraintKind = iota
	kindPercent
	kindRatio
	kindMin
	kindMax
	kindFill
)

// Constraint describes how one segment of a Split should be sized along
// the split's axis. The zero Constraint is Length(0). See Length,
// Percent, Ratio, Min, Max, and Fill for the ways to construct one.
type Constraint struct {
	kind  constraintKind
	value int // Length: cells. Percent: 0-100. Min/Max: cells. Fill: weight.
	numer int // Ratio numerator
	denom int // Ratio denominator
}

// Length is a fixed size of exactly n cells.
func Length(n int) Constraint { return Constraint{kind: kindLength, value: n} }

// Percent is p percent of the available space along the split axis
// (rounded to the nearest cell).
func Percent(p int) Constraint { return Constraint{kind: kindPercent, value: p} }

// Ratio is num/den of the available space along the split axis
// (rounded down).
func Ratio(num, den int) Constraint { return Constraint{kind: kindRatio, numer: num, denom: den} }

// Min is a flexible segment that shares leftover space with other
// flexible segments (the same as Fill(1)), but is never sized below n
// cells even if that leftover share would otherwise be smaller.
func Min(n int) Constraint { return Constraint{kind: kindMin, value: n} }

// Max is a flexible segment that shares leftover space with other
// flexible segments (the same as Fill(1)), but is never sized above n
// cells — space it would otherwise have taken is redistributed to the
// other flexible segments.
func Max(n int) Constraint { return Constraint{kind: kindMax, value: n} }

// Fill is a flexible segment that shares whatever space remains after
// every Length/Percent/Ratio segment has been sized, proportionally to
// weight against the other Fill/Min/Max segments in the same split
// (Min and Max both count as weight 1). A non-positive weight is
// treated as 1.
func Fill(weight int) Constraint { return Constraint{kind: kindFill, value: weight} }
