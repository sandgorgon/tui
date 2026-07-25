// Package vt implements a VT100/xterm-compatible terminal emulator: the
// classic Paul Williams / DEC VT500 parser state machine driving a
// Screen model (primary + alt buffers, scrollback, cursor, SGR
// attributes, scrolling regions, tab stops, OSC title/hyperlink, and
// DA/DSR query responses).
//
// Scope is deliberately "correct enough to run bash/zsh/vim/htop/less/
// nested tmux," not a bug-for-bug xterm clone. Screen exposes a
// *cell.Buffer so emulator output composes directly with the rest of
// the rendering pipeline.
//
// See docs/DESIGN.md §7 for the design.
package vt
