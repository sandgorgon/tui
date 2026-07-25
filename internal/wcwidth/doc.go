// Package wcwidth provides East Asian width and emoji width lookups for
// rune display width, via hand-generated Unicode range tables (there is
// no dependency on golang.org/x/text/width). See the package-level
// regeneration script referenced in docs/DESIGN.md §9 for how the
// tables are kept current with Unicode updates.
package wcwidth
