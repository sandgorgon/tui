// Package layout implements a one-pass flex-style layout solver over
// Constraint values (Length, Percent, Ratio, Min, Max, Fill), producing
// nested Rects — deliberately not a general iterative constraint solver,
// since layout reruns on every redraw.
//
// Split (or the Layout builder, for Gap/Margin) turns a Rect and a list
// of Constraints into one Rect per constraint, adjacent along a
// Direction. Nesting is just calling Split again on a Rect it returned.
//
// See docs/DESIGN.md §3.3 for the design.
package layout
