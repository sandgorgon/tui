package term

import "os"

// Size holds a terminal's dimensions in character cells.
type Size struct {
	Rows int
	Cols int
}

// withFd runs fn with the raw file descriptor underlying f, via
// SyscallConn rather than f.Fd(). This matters: f.Fd() permanently
// disables f's SetReadDeadline (documented Unix/Windows behavior in the
// os package), which would silently break term.Probe and any
// input.Decoder reading from the same *os.File afterward. SyscallConn's
// Control callback hands back the fd for the duration of one syscall
// without that side effect.
func withFd(f *os.File, fn func(fd int) error) error {
	conn, err := f.SyscallConn()
	if err != nil {
		return err
	}
	var opErr error
	if err := conn.Control(func(fd uintptr) {
		opErr = fn(int(fd))
	}); err != nil {
		return err
	}
	return opErr
}
