// Package pty allocates and manages Unix pseudo-terminals: master/slave
// pair allocation, attaching a subprocess as the controlling terminal,
// resize (TIOCSWINSZ + SIGWINCH forwarding), raw/cooked mode toggling on
// the host side, and signal forwarding (SIGINT/SIGTSTP/SIGCONT) to the
// child process group.
//
// Implemented entirely with the standard library's syscall package
// (build-tagged per GOOS: pty_linux.go, pty_darwin.go) — no cgo,
// no golang.org/x/sys.
//
// See docs/DESIGN.md §6 for the design.
package pty
