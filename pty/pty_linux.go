//go:build linux

package pty

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
	"unsafe"

	"github.com/sandgorgon/tui/term"
)

// ioctl request numbers for the Unix98 /dev/ptmx allocation protocol,
// from /usr/include/asm-generic/ioctls.h (the same source used and
// empirically verified against a real pty in package term's tests —
// see term/term_linux_test.go).
const (
	tiocgptn   = 0x80045430
	tiocsptlck = 0x40045431
)

func ioctl(fd int, req uintptr, arg unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}

// openpty allocates a Unix98 pty pair: open /dev/ptmx for the master,
// unlock it (TIOCSPTLCK) and read its number (TIOCGPTN), then open the
// corresponding /dev/pts/<n> for the slave.
func openpty() (master, slave *os.File, err error) {
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("pty: open /dev/ptmx: %w", err)
	}

	var n int32
	txErr := term.WithFd(m, func(fd int) error {
		var unlock int32
		if err := ioctl(fd, tiocsptlck, unsafe.Pointer(&unlock)); err != nil {
			return fmt.Errorf("TIOCSPTLCK: %w", err)
		}
		if err := ioctl(fd, tiocgptn, unsafe.Pointer(&n)); err != nil {
			return fmt.Errorf("TIOCGPTN: %w", err)
		}
		return nil
	})
	if txErr != nil {
		m.Close()
		return nil, nil, fmt.Errorf("pty: %w", txErr)
	}

	s, err := os.OpenFile("/dev/pts/"+strconv.Itoa(int(n)), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		m.Close()
		return nil, nil, fmt.Errorf("pty: open slave: %w", err)
	}

	return m, s, nil
}
