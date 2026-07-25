//go:build darwin

package term

import (
	"os"
	"syscall"
	"unsafe"
)

// ioctl request numbers. Unlike Linux's fixed values, BSD/Darwin ioctl
// numbers are computed from the request's direction, group, number and
// argument size, which makes them easy to get subtly wrong by
// hand-deriving; these are taken verbatim from golang.org/x/sys/unix's
// zerrors_darwin_amd64.go (also used unmodified on arm64/Apple Silicon,
// which shares the same ioctl ABI), not re-derived from the _IOC macros.
const (
	tiocgeta   = 0x40487413
	tiocseta   = 0x80487414
	tiocgwinsz = 0x40087468
	tiocswinsz = 0x80087467
)

// termios mirrors Darwin's struct termios (sys/termios.h) exactly as
// read/written by the TIOCGETA/TIOCSETA ioctls: on Darwin, tcflag_t is
// widened to 64 bits and the struct ends with 64-bit input/output speed
// fields, unlike Linux's narrower, speed-less kernel termios. Field
// widths and ordering are taken from x/sys/unix's ztypes_darwin_amd64.go.
type termios struct {
	Iflag  uint64
	Oflag  uint64
	Cflag  uint64
	Lflag  uint64
	Cc     [20]uint8
	Ispeed uint64
	Ospeed uint64
}

// Flag bits and c_cc indices, from x/sys/unix's zerrors_darwin_amd64.go
// (ultimately <sys/termios.h>) — note these bit positions and control
// character indices differ from Linux's.
const (
	iIgnbrk = 0x00000001
	iBrkint = 0x00000002
	iParmrk = 0x00000008
	iIstrip = 0x00000020
	iInlcr  = 0x00000040
	iIgncr  = 0x00000080
	iIcrnl  = 0x00000100
	iIxon   = 0x00000200

	oOpost = 0x00000001

	cCsize  = 0x00000300
	cCs8    = 0x00000300
	cParenb = 0x00001000

	lIsig   = 0x00000080
	lIcanon = 0x00000100
	lEcho   = 0x00000008
	lEchonl = 0x00000010
	lIexten = 0x00000400

	vmin  = 16
	vtime = 17
)

type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

func ioctl(fd int, req uintptr, arg unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}

func getTermios(fd int) (*termios, error) {
	var t termios
	if err := ioctl(fd, tiocgeta, unsafe.Pointer(&t)); err != nil {
		return nil, err
	}
	return &t, nil
}

func setTermios(fd int, t *termios) error {
	return ioctl(fd, tiocseta, unsafe.Pointer(t))
}

// State is a terminal's saved termios state, for restoring after MakeRaw.
type State struct {
	termios termios
}

// IsTerminal reports whether f refers to a terminal.
func IsTerminal(f *os.File) bool {
	ok := false
	_ = withFd(f, func(fd int) error {
		_, err := getTermios(fd)
		ok = err == nil
		return nil
	})
	return ok
}

// GetSize returns the terminal's current dimensions.
func GetSize(f *os.File) (Size, error) {
	var size Size
	err := withFd(f, func(fd int) error {
		var ws winsize
		if err := ioctl(fd, tiocgwinsz, unsafe.Pointer(&ws)); err != nil {
			return err
		}
		size = Size{Rows: int(ws.Row), Cols: int(ws.Col)}
		return nil
	})
	return size, err
}

// SetSize sets a terminal's dimensions. It's meaningful on a pty slave
// (see package pty), not on the host's own real terminal.
func SetSize(f *os.File, s Size) error {
	return withFd(f, func(fd int) error {
		ws := winsize{Row: uint16(s.Rows), Col: uint16(s.Cols)}
		return ioctl(fd, tiocswinsz, unsafe.Pointer(&ws))
	})
}

// GetState returns the terminal's current termios state without
// changing it.
func GetState(f *os.File) (*State, error) {
	var state State
	err := withFd(f, func(fd int) error {
		t, err := getTermios(fd)
		if err != nil {
			return err
		}
		state = State{termios: *t}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &state, nil
}

// MakeRaw puts the terminal into raw mode, replicating the behavior
// documented for cfmakeraw(3), and returns the previous state so it can
// later be restored with Restore.
func MakeRaw(f *os.File) (*State, error) {
	var prev State
	err := withFd(f, func(fd int) error {
		t, err := getTermios(fd)
		if err != nil {
			return err
		}
		prev = State{termios: *t}

		t.Iflag &^= iIgnbrk | iBrkint | iParmrk | iIstrip | iInlcr | iIgncr | iIcrnl | iIxon
		t.Oflag &^= oOpost
		t.Lflag &^= lEcho | lEchonl | lIcanon | lIsig | lIexten
		t.Cflag &^= cCsize | cParenb
		t.Cflag |= cCs8
		t.Cc[vmin] = 1
		t.Cc[vtime] = 0

		return setTermios(fd, t)
	})
	if err != nil {
		return nil, err
	}
	return &prev, nil
}

// Restore restores a terminal to a previously saved state.
func Restore(f *os.File, state *State) error {
	return withFd(f, func(fd int) error {
		t := state.termios
		return setTermios(fd, &t)
	})
}
