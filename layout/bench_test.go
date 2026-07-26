package layout

import "testing"

// BenchmarkSplitMixedConstraints exercises the solver's full
// constraint mix (Length, Fill, Min, Max, Percent) at once, nested
// (an outer vertical split with an inner horizontal split of one
// segment) — closer to what a real app's frame layout looks like than
// a single-constraint-kind split.
func BenchmarkSplitMixedConstraints(b *testing.B) {
	area := Rect{X: 0, Y: 0, W: 120, H: 40}
	for b.Loop() {
		rows := New(Vertical, Length(1), Fill(1), Length(1)).Gap(1).Margin(1).Split(area)
		_ = New(Horizontal, Min(10), Percent(30), Max(40), Fill(2)).Gap(1).Split(rows[1])
	}
}
