package layout

import (
	"reflect"
	"testing"
)

func TestSplitLength(t *testing.T) {
	got := Split(Horizontal, Rect{W: 10, H: 5}, Length(3), Length(7))
	want := []Rect{
		{X: 0, Y: 0, W: 3, H: 5},
		{X: 3, Y: 0, W: 7, H: 5},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Split = %+v, want %+v", got, want)
	}
}

func TestSplitPercent(t *testing.T) {
	got := Split(Horizontal, Rect{W: 10, H: 1}, Percent(30), Percent(70))
	want := []Rect{
		{X: 0, Y: 0, W: 3, H: 1},
		{X: 3, Y: 0, W: 7, H: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Split = %+v, want %+v", got, want)
	}
}

func TestSplitRatio(t *testing.T) {
	got := Split(Horizontal, Rect{W: 9, H: 1}, Ratio(1, 3), Ratio(2, 3))
	want := []Rect{
		{X: 0, Y: 0, W: 3, H: 1},
		{X: 3, Y: 0, W: 6, H: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Split = %+v, want %+v", got, want)
	}
}

func TestSplitFillEqual(t *testing.T) {
	got := Split(Horizontal, Rect{W: 10, H: 1}, Fill(1), Fill(1))
	want := []Rect{
		{X: 0, Y: 0, W: 5, H: 1},
		{X: 5, Y: 0, W: 5, H: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Split = %+v, want %+v", got, want)
	}
}

func TestSplitFillWeightedWithLargestRemainder(t *testing.T) {
	// remaining after Length(10) is 30, split 1:2 -> 10, 20 exactly.
	got := Split(Horizontal, Rect{W: 40, H: 1}, Length(10), Fill(1), Fill(2))
	want := []Rect{
		{X: 0, Y: 0, W: 10, H: 1},
		{X: 10, Y: 0, W: 10, H: 1},
		{X: 20, Y: 0, W: 20, H: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Split = %+v, want %+v", got, want)
	}

	// remaining 10 split 1:1:1 doesn't divide evenly (3,3,4); every
	// segment must still sum to the total available space.
	got2 := Split(Horizontal, Rect{W: 10, H: 1}, Fill(1), Fill(1), Fill(1))
	sum := 0
	for _, r := range got2 {
		if r.W < 3 || r.W > 4 {
			t.Errorf("Fill(1)x3 over 10 cells produced W=%d, want 3 or 4", r.W)
		}
		sum += r.W
	}
	if sum != 10 {
		t.Errorf("Fill(1)x3 widths sum to %d, want 10", sum)
	}
}

func TestSplitMinFloor(t *testing.T) {
	// Fill(1) would get an equal 5/5 share of 10, but Min(8) forces its
	// segment to 8, leaving only 2 for the plain Fill(1).
	got := Split(Horizontal, Rect{W: 10, H: 1}, Min(8), Fill(1))
	want := []Rect{
		{X: 0, Y: 0, W: 8, H: 1},
		{X: 8, Y: 0, W: 2, H: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Split = %+v, want %+v", got, want)
	}
}

func TestSplitMaxCeiling(t *testing.T) {
	// Fill(1) would get an equal 5/5 share of 10, but Max(2) caps its
	// segment, and the freed 3 cells go to the plain Fill(1).
	got := Split(Horizontal, Rect{W: 10, H: 1}, Max(2), Fill(1))
	want := []Rect{
		{X: 0, Y: 0, W: 2, H: 1},
		{X: 2, Y: 0, W: 8, H: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Split = %+v, want %+v", got, want)
	}
}

func TestSplitVerticalDirection(t *testing.T) {
	got := Split(Vertical, Rect{W: 20, H: 10}, Length(2), Fill(1))
	want := []Rect{
		{X: 0, Y: 0, W: 20, H: 2},
		{X: 0, Y: 2, W: 20, H: 8},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Split = %+v, want %+v", got, want)
	}
}

func TestSplitGap(t *testing.T) {
	got := New(Horizontal, Length(3), Length(3)).Gap(2).Split(Rect{W: 10, H: 1})
	want := []Rect{
		{X: 0, Y: 0, W: 3, H: 1},
		{X: 5, Y: 0, W: 3, H: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Split = %+v, want %+v", got, want)
	}
}

func TestSplitMargin(t *testing.T) {
	got := New(Horizontal, Fill(1), Fill(1)).Margin(1).Split(Rect{X: 0, Y: 0, W: 12, H: 5})
	want := []Rect{
		{X: 1, Y: 1, W: 5, H: 3},
		{X: 6, Y: 1, W: 5, H: 3},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Split = %+v, want %+v", got, want)
	}
}

func TestSplitOverflowFixedConstraintsClamped(t *testing.T) {
	// Length(6)+Length(6) = 12 exceeds the 10-cell area; both must be
	// scaled down proportionally so the sum still fits exactly.
	got := Split(Horizontal, Rect{W: 10, H: 1}, Length(6), Length(6))
	sum := 0
	for _, r := range got {
		sum += r.W
	}
	if sum != 10 {
		t.Errorf("overflowing Lengths sum to %d, want 10", sum)
	}
	if got[0].W != got[1].W {
		t.Errorf("equal Length(6) constraints clamped unevenly: %+v", got)
	}
}

func TestSplitZeroConstraints(t *testing.T) {
	got := Split(Horizontal, Rect{W: 10, H: 5})
	if len(got) != 0 {
		t.Errorf("Split with no constraints = %+v, want empty", got)
	}
}

func TestSplitAreaSmallerThanMargin(t *testing.T) {
	got := New(Horizontal, Fill(1)).Margin(10).Split(Rect{W: 4, H: 4})
	want := []Rect{{X: 10, Y: 10, W: 0, H: 0}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Split = %+v, want %+v", got, want)
	}
}

func TestSplitNesting(t *testing.T) {
	rows := Split(Vertical, Rect{W: 20, H: 10}, Length(1), Fill(1))
	body := rows[1]
	if body != (Rect{X: 0, Y: 1, W: 20, H: 9}) {
		t.Fatalf("body row = %+v", body)
	}

	cols := Split(Horizontal, body, Length(5), Fill(1))
	want := []Rect{
		{X: 0, Y: 1, W: 5, H: 9},
		{X: 5, Y: 1, W: 15, H: 9},
	}
	if !reflect.DeepEqual(cols, want) {
		t.Errorf("nested Split = %+v, want %+v", cols, want)
	}
}

func TestDistributeSumsToTotal(t *testing.T) {
	cases := []struct {
		total   int
		weights []int
	}{
		{10, []int{1, 1, 1}},
		{100, []int{1, 2, 3, 4}},
		{7, []int{5, 1}},
		{0, []int{1, 1}},
		{5, []int{}},
		{5, []int{0, 0}},
	}
	for _, c := range cases {
		sizes := distribute(c.total, c.weights)
		if len(sizes) != len(c.weights) {
			t.Errorf("distribute(%d, %v) len = %d, want %d", c.total, c.weights, len(sizes), len(c.weights))
			continue
		}
		sum := 0
		for _, s := range sizes {
			if s < 0 {
				t.Errorf("distribute(%d, %v) produced negative size %v", c.total, c.weights, sizes)
			}
			sum += s
		}
		wantSum := c.total
		sumW := 0
		for _, w := range c.weights {
			sumW += w
		}
		if sumW <= 0 || c.total <= 0 {
			wantSum = 0
		}
		if sum != wantSum {
			t.Errorf("distribute(%d, %v) = %v, sums to %d, want %d", c.total, c.weights, sizes, sum, wantSum)
		}
	}
}
