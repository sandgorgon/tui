//go:build darwin

package pty

import (
	"bytes"
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"github.com/sandgorgon/tui/term"
)

// ioctl request numbers for Darwin's /dev/ptmx allocation protocol —
// grantpt/unlockpt/ptsname's kernel-level equivalents. Taken verbatim
// from golang.org/x/sys/unix's zerrors_darwin_amd64.go, the same
// verification approach used for term/term_darwin.go's constants (this
// sandbox is Linux-only, so these are unverified on real Darwin
// hardware — see docs/DESIGN.md §9).
const (
	tiocptygrant = 0x20007454 // IOC_VOID: grant access to the slave
	tiocptyunlk  = 0x20007452 // IOC_VOID: unlock the slave for opening
	tiocptygname = 0x40807453 // IOC_OUT, 128-byte buffer: the slave's device path
)

func ioctl(fd int, req uintptr, arg unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}

// openpty allocates a Darwin pty pair: open /dev/ptmx for the master,
// grant and unlock the slave (TIOCPTYGRANT, TIOCPTYUNLK), read its
// device path (TIOCPTYGNAME), then open that path for the slave.
func openpty() (master, slave *os.File, err error) {
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("pty: open /dev/ptmx: %w", err)
	}

	var nameBuf [128]byte
	txErr := term.WithFd(m, func(fd int) error {
		if err := ioctl(fd, tiocptygrant, nil); err != nil {
			return fmt.Errorf("TIOCPTYGRANT: %w", err)
		}
		if err := ioctl(fd, tiocptyunlk, nil); err != nil {
			return fmt.Errorf("TIOCPTYUNLK: %w", err)
		}
		if err := ioctl(fd, tiocptygname, unsafe.Pointer(&nameBuf[0])); err != nil {
			return fmt.Errorf("TIOCPTYGNAME: %w", err)
		}
		return nil
	})
	if txErr != nil {
		m.Close()
		return nil, nil, fmt.Errorf("pty: %w", txErr)
	}

	end := bytes.IndexByte(nameBuf[:], 0)
	if end < 0 {
		end = len(nameBuf)
	}
	name := string(nameBuf[:end])

	s, err := os.OpenFile(name, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		m.Close()
		return nil, nil, fmt.Errorf("pty: open slave %s: %w", name, err)
	}

	return m, s, nil
}
