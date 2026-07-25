package main

// computeLayout splits a hostW x hostH terminal into two side-by-side
// panes separated by a one-column divider. This is deliberately
// hand-rolled rather than using a real layout engine — package layout
// doesn't exist yet (M7); this milestone's job is proving pty+vt+
// render work together end to end, not building layout early.
func computeLayout(hostW, hostH int) (left, right Rect) {
	if hostW < 3 {
		return Rect{0, 0, hostW, hostH}, Rect{0, 0, 0, hostH}
	}
	leftW := (hostW - 1) / 2
	rightW := hostW - 1 - leftW
	return Rect{0, 0, leftW, hostH}, Rect{leftW + 1, 0, rightW, hostH}
}
