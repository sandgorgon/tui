// Package render diffs successive cell.Buffer frames and writes the
// minimal ANSI/SGR byte sequence needed to bring the terminal from one
// frame to the next, using a cost-based (not just minimal) span-merge
// diff, with DEC synchronized-update wrapping where supported.
//
// The render -> vt round trip (render a Buffer to bytes, parse those
// bytes back with package vt, assert equality with the source Buffer)
// is the project's primary rendering-correctness harness; see
// docs/DESIGN.md §3.2 and §10.
package render
