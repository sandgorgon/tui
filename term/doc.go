// Package term controls the host terminal: raw/cooked mode via termios
// ioctls, terminal capability probing without a terminfo database, and
// SIGWINCH/SIGCONT/SIGTSTP handling.
//
// See docs/DESIGN.md §3.4 and §4 for the design.
package term
