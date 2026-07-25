package layout

// Rect is an axis-aligned rectangle in cell coordinates. It's package
// layout's own minimal type for this — structurally identical to, but
// independent of, cell.Rect (see cell/painter.go): cell sits below
// layout in the architecture (docs/DESIGN.md §3), so it can't depend on
// this package. Widgets convert between the two at the boundary, a
// trivial field copy.
type Rect struct {
	X, Y, W, H int
}
