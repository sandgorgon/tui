package layout

import "sort"

// Direction is the axis a Layout splits along.
type Direction int

const (
	// Horizontal splits left-to-right; constraints size each segment's
	// width, and every segment spans the full height of the area.
	Horizontal Direction = iota
	// Vertical splits top-to-bottom; constraints size each segment's
	// height, and every segment spans the full width of the area.
	Vertical
)

// Layout splits a Rect into adjacent sub-Rects along one axis according
// to a list of Constraints, optionally separated by a Gap and inset by
// a Margin. It's a one-pass flex solver, not a general iterative
// constraint solver (see docs/DESIGN.md §3.3) — deliberately, since
// layout reruns on every redraw. Split results nest naturally: pass a
// Rect returned by one Split into another Layout's Split to subdivide
// it further.
//
// A Layout is an immutable value; Gap and Margin return a modified
// copy, so it's safe to build once and reuse (or share a base Layout
// across call sites via With-style chaining).
type Layout struct {
	direction   Direction
	constraints []Constraint
	gap         int
	margin      int
}

// New returns a Layout that splits along direction using constraints,
// applied in order.
func New(direction Direction, constraints ...Constraint) Layout {
	return Layout{direction: direction, constraints: constraints}
}

// Split is a convenience for New(direction, constraints...).Split(area)
// — building a Layout inline when Gap/Margin aren't needed.
func Split(direction Direction, area Rect, constraints ...Constraint) []Rect {
	return New(direction, constraints...).Split(area)
}

// Gap returns a copy of l with n cells of empty space inserted between
// each adjacent pair of segments (none before the first or after the
// last).
func (l Layout) Gap(n int) Layout {
	l.gap = n
	return l
}

// Margin returns a copy of l with n cells of empty space inset from
// every side of the area before constraints are applied.
func (l Layout) Margin(n int) Layout {
	l.margin = n
	return l
}

// Split computes one Rect per constraint, positioned adjacently along
// l's direction and inset by l's margin, and returns them in
// constraint order. The returned slice always has the same length as
// l.constraints, even when area is too small to hold everything (in
// which case segments are clamped down to fit rather than overflowing
// area).
func (l Layout) Split(area Rect) []Rect {
	n := len(l.constraints)
	out := make([]Rect, n)
	if n == 0 {
		return out
	}

	inner := Rect{
		X: area.X + l.margin,
		Y: area.Y + l.margin,
		W: max(area.W-2*l.margin, 0),
		H: max(area.H-2*l.margin, 0),
	}

	axisLen := inner.W
	if l.direction == Vertical {
		axisLen = inner.H
	}
	total := max(axisLen-l.gap*max(n-1, 0), 0)

	sizes := l.solve(total)

	pos := 0
	for i, sz := range sizes {
		if l.direction == Horizontal {
			out[i] = Rect{X: inner.X + pos, Y: inner.Y, W: sz, H: inner.H}
		} else {
			out[i] = Rect{X: inner.X, Y: inner.Y + pos, W: inner.W, H: sz}
		}
		pos += sz + l.gap
	}
	return out
}

// flexItem is a Min/Max/Fill constraint after the fixed (Length/
// Percent/Ratio) constraints have been sized, awaiting its share of
// whatever space remains.
type flexItem struct {
	constraintIdx  int
	weight         int
	min, max       int
	hasMin, hasMax bool
}

// solve computes one size per constraint (in constraint order) that
// sum to exactly total, in two passes: fixed constraints (Length/
// Percent/Ratio) are sized directly against total, then whatever space
// remains is shared among the flexible constraints (Fill/Min/Max) by
// weight, with a single follow-up pass to redistribute space freed by
// Max caps or consumed by Min floors. It's bounded work, not iterated
// to convergence (see docs/DESIGN.md §3.3); pathological constraint
// sets (e.g. Min floors that alone exceed total) are clamped rather
// than solved exactly.
func (l Layout) solve(total int) []int {
	sizes := make([]int, len(l.constraints))
	isFixed := make([]bool, len(l.constraints))
	fixedSum := 0

	for i, c := range l.constraints {
		switch c.kind {
		case kindLength:
			isFixed[i] = true
			sizes[i] = max(c.value, 0)
		case kindPercent:
			isFixed[i] = true
			sizes[i] = max(roundDiv(total*c.value, 100), 0)
		case kindRatio:
			den := c.denom
			if den == 0 {
				den = 1
			}
			isFixed[i] = true
			sizes[i] = max(total*c.numer/den, 0)
		}
		if isFixed[i] {
			fixedSum += sizes[i]
		}
	}

	if fixedSum > total {
		var idxs, weights []int
		for i := range l.constraints {
			if isFixed[i] {
				idxs = append(idxs, i)
				weights = append(weights, sizes[i])
			} else {
				sizes[i] = 0
			}
		}
		for j, sz := range distribute(total, weights) {
			sizes[idxs[j]] = sz
		}
		return sizes
	}

	var flex []flexItem
	for i, c := range l.constraints {
		if isFixed[i] {
			continue
		}
		switch c.kind {
		case kindMin:
			flex = append(flex, flexItem{constraintIdx: i, weight: 1, min: c.value, hasMin: true})
		case kindMax:
			flex = append(flex, flexItem{constraintIdx: i, weight: 1, max: c.value, hasMax: true})
		default: // kindFill
			w := c.value
			if w <= 0 {
				w = 1
			}
			flex = append(flex, flexItem{constraintIdx: i, weight: w})
		}
	}
	if len(flex) == 0 {
		return sizes
	}

	remaining := total - fixedSum
	weights := make([]int, len(flex))
	for i, f := range flex {
		weights[i] = f.weight
	}
	shares := distribute(remaining, weights)

	clamped := make([]bool, len(flex))
	clampedSum := 0
	for i, f := range flex {
		switch {
		case f.hasMin && shares[i] < f.min:
			sizes[f.constraintIdx] = f.min
			clamped[i] = true
			clampedSum += f.min
		case f.hasMax && shares[i] > f.max:
			sizes[f.constraintIdx] = f.max
			clamped[i] = true
			clampedSum += f.max
		}
	}

	var unclamped []int
	for i := range flex {
		if !clamped[i] {
			unclamped = append(unclamped, i)
		}
	}
	if len(unclamped) > 0 {
		pool := max(remaining-clampedSum, 0)
		uw := make([]int, len(unclamped))
		for j, i := range unclamped {
			uw[j] = flex[i].weight
		}
		final := distribute(pool, uw)
		for j, i := range unclamped {
			sizes[flex[i].constraintIdx] = final[j]
		}
	}

	return sizes
}

// distribute splits total into len(weights) non-negative integers
// proportional to weights, summing to exactly total (assuming total
// >= 0). It uses the largest-remainder method: each part first gets
// its rounded-down proportional share, then the parts with the largest
// truncated remainders each get one more cell until the sum matches
// total exactly — so rounding error is spread out rather than lost or
// piled onto one segment.
func distribute(total int, weights []int) []int {
	sizes := make([]int, len(weights))
	if total <= 0 || len(weights) == 0 {
		return sizes
	}
	sumW := 0
	for _, w := range weights {
		sumW += w
	}
	if sumW <= 0 {
		return sizes
	}

	type remainder struct {
		idx  int
		frac int // numerator of the fractional remainder, out of sumW
	}
	rems := make([]remainder, len(weights))
	allocated := 0
	for i, w := range weights {
		exact := total * w
		base := exact / sumW
		sizes[i] = base
		rems[i] = remainder{idx: i, frac: exact - base*sumW}
		allocated += base
	}

	sort.SliceStable(rems, func(a, b int) bool { return rems[a].frac > rems[b].frac })
	for i := 0; i < total-allocated && i < len(rems); i++ {
		sizes[rems[i].idx]++
	}
	return sizes
}

// roundDiv computes round(a/b) for b > 0, rounding half away from
// zero.
func roundDiv(a, b int) int {
	if b == 0 {
		return 0
	}
	half := b / 2
	if a >= 0 {
		return (a + half) / b
	}
	return -((-a + half) / b)
}
