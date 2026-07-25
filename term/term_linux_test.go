//go:build linux

package term

import (
	"os"
	"strconv"
	"testing"
	"unsafe"
)

// ioctl request numbers for bootstrapping a Unix98 pty pair via
// /dev/ptmx, from /usr/include/asm-generic/ioctls.h. Package pty (M3)
// will own this properly; this is a minimal, test-only bootstrap so the
// term package's ioctl code can be verified against a real tty here and
// now rather than deferred to manual testing.
const (
	tiocgptn   = 0x80045430
	tiocsptlck = 0x40045431
)

func openTestPTY(t *testing.T) (master, slave *os.File) {
	t.Helper()

	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		t.Skipf("no /dev/ptmx available: %v", err)
	}
	t.Cleanup(func() { m.Close() })

	var unlock int32
	if err := ioctl(int(m.Fd()), tiocsptlck, unsafe.Pointer(&unlock)); err != nil {
		t.Fatalf("TIOCSPTLCK: %v", err)
	}

	var n int32
	if err := ioctl(int(m.Fd()), tiocgptn, unsafe.Pointer(&n)); err != nil {
		t.Fatalf("TIOCGPTN: %v", err)
	}

	s, err := os.OpenFile("/dev/pts/"+strconv.Itoa(int(n)), os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open pty slave %d: %v", n, err)
	}
	t.Cleanup(func() { s.Close() })

	return m, s
}

func TestIsTerminal(t *testing.T) {
	_, slave := openTestPTY(t)
	if !IsTerminal(slave) {
		t.Error("IsTerminal(pty slave) = false, want true")
	}

	f, err := os.Open("/dev/null")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if IsTerminal(f) {
		t.Error("IsTerminal(/dev/null) = true, want false")
	}
}

func TestGetSetSize(t *testing.T) {
	_, slave := openTestPTY(t)

	want := Size{Rows: 40, Cols: 120}
	if err := SetSize(slave, want); err != nil {
		t.Fatalf("SetSize: %v", err)
	}
	got, err := GetSize(slave)
	if err != nil {
		t.Fatalf("GetSize: %v", err)
	}
	if got != want {
		t.Errorf("GetSize = %+v, want %+v", got, want)
	}
}

func TestMakeRawRestore(t *testing.T) {
	_, slave := openTestPTY(t)

	before, err := GetState(slave)
	if err != nil {
		t.Fatalf("GetState: %v", err)
	}
	if before.termios.Lflag&lIcanon == 0 {
		t.Fatalf("precondition failed: pty slave not in canonical mode before MakeRaw")
	}

	saved, err := MakeRaw(slave)
	if err != nil {
		t.Fatalf("MakeRaw: %v", err)
	}
	if saved.termios != before.termios {
		t.Error("MakeRaw's saved previous state doesn't match the pre-raw state")
	}

	raw, err := GetState(slave)
	if err != nil {
		t.Fatalf("GetState after MakeRaw: %v", err)
	}
	if raw.termios.Lflag&(lIcanon|lEcho|lIsig|lIexten) != 0 {
		t.Errorf("Lflag = %#x, want ICANON|ECHO|ISIG|IEXTEN all clear", raw.termios.Lflag)
	}
	if raw.termios.Iflag&iIcrnl != 0 {
		t.Errorf("Iflag = %#x, want ICRNL clear", raw.termios.Iflag)
	}
	if raw.termios.Oflag&oOpost != 0 {
		t.Errorf("Oflag = %#x, want OPOST clear", raw.termios.Oflag)
	}
	if raw.termios.Cflag&cCsize != cCs8 {
		t.Errorf("Cflag&CSIZE = %#x, want CS8", raw.termios.Cflag&cCsize)
	}
	if raw.termios.Cc[vmin] != 1 || raw.termios.Cc[vtime] != 0 {
		t.Errorf("Cc[VMIN]=%d Cc[VTIME]=%d, want 1, 0", raw.termios.Cc[vmin], raw.termios.Cc[vtime])
	}

	if err := Restore(slave, saved); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	after, err := GetState(slave)
	if err != nil {
		t.Fatalf("GetState after Restore: %v", err)
	}
	if after.termios != before.termios {
		t.Errorf("state after Restore = %+v, want %+v (the original)", after.termios, before.termios)
	}
}
