package main

// computeLayout splits a hostW x hostH terminal into two side-by-side
// panes separated by a one-column divider. This predates package
// layout (M7) and is deliberately still hand-rolled rather than
// switched over to it: this example's job was proving pty+vt+render
// work together end to end (M6), not layout.
func computeLayout(hostW, hostH int) (left, right Rect) {
	if hostW < 3 {
		return Rect{0, 0, hostW, hostH}, Rect{0, 0, 0, hostH}
	}
	leftW := (hostW - 1) / 2
	rightW := hostW - 1 - leftW
	return Rect{0, 0, leftW, hostH}, Rect{leftW + 1, 0, rightW, hostH}
}
