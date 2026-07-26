// Package style is a theming layer built on cell.Color/cell.Style —
// not a redefinition of them, which live in package cell as of M2
// (cell.Cell needs a concrete Style type three layers below where
// style sits; see docs/DESIGN.md §4). It provides named/semantic
// colors via Theme, adaptive light/dark palettes (DefaultDark/
// DefaultLight/DetectAppearance), and Builder, a small fluent
// cell.Style constructor — the API the widget catalog (M10/M11)
// builds on rather than working with cell.Style literals directly.
//
// See docs/DESIGN.md §4 for the design.
package style
