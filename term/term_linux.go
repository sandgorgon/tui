//go:build linux

package term

import (
	"os"
	"syscall"
	"unsafe"
)

// ioctl request numbers, from /usr/include/asm-generic/ioctls.h.
const (
	tcgets     = 0x5401
	tcsets     = 0x5402
	tiocgwinsz = 0x5413
	tiocswinsz = 0x5414
)

// termios mirrors the Linux kernel's struct termios (asm-generic/termbits.h)
// exactly as read/written by the TCGETS/TCSETS ioctls: 4 uint32 mode
// flags, a line-discipline byte, and 19 control-character bytes — no
// trailing speed fields (those belong to the separate termios2/TCGETS2
// ABI, which this package doesn't use).
type termios struct {
	Iflag uint32
	Oflag uint32
	Cflag uint32
	Lflag uint32
	Line  uint8
	Cc    [19]uint8
}

// Flag bits, from /usr/include/asm-generic/termbits.h and termbits-common.h.
const (
	iIgnbrk = 0x001
	iBrkint = 0x002
	iParmrk = 0x008
	iIstrip = 0x020
	iInlcr  = 0x040
	iIgncr  = 0x080
	iIcrnl  = 0x100
	iIxon   = 0x400

	oOpost = 0x01

	cCsize  = 0x00000030
	cCs8    = 0x00000030
	cParenb = 0x00000100

	lIsig   = 0x00001
	lIcanon = 0x00002
	lEcho   = 0x00008
	lEchonl = 0x00040
	lIexten = 0x08000

	vmin  = 6
	vtime = 5
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
	if err := ioctl(fd, tcgets, unsafe.Pointer(&t)); err != nil {
		return nil, err
	}
	return &t, nil
}

func setTermios(fd int, t *termios) error {
	return ioctl(fd, tcsets, unsafe.Pointer(t))
}

// State is a terminal's saved termios state, for restoring after MakeRaw.
type State struct {
	termios termios
}

// IsTerminal reports whether f refers to a terminal.
func IsTerminal(f *os.File) bool {
	ok := false
	_ = WithFd(f, func(fd int) error {
		_, err := getTermios(fd)
		ok = err == nil
		return nil
	})
	return ok
}

// GetSize returns the terminal's current dimensions.
func GetSize(f *os.File) (Size, error) {
	var size Size
	err := WithFd(f, func(fd int) error {
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
	return WithFd(f, func(fd int) error {
		ws := winsize{Row: uint16(s.Rows), Col: uint16(s.Cols)}
		return ioctl(fd, tiocswinsz, unsafe.Pointer(&ws))
	})
}

// GetState returns the terminal's current termios state without
// changing it.
func GetState(f *os.File) (*State, error) {
	var state State
	err := WithFd(f, func(fd int) error {
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
	err := WithFd(f, func(fd int) error {
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
	return WithFd(f, func(fd int) error {
		t := state.termios
		return setTermios(fd, &t)
	})
}
