// Package layout implements a one-pass flex-style layout solver over
// Constraint values (Length, Percent, Ratio, Min, Max, Fill), producing
// nested Rects — deliberately not a general iterative constraint solver,
// since layout reruns on every redraw.
//
// See docs/DESIGN.md §3.3 for the design.
package layout
