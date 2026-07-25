package pty_test

import (
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/sandgorgon/tui/pty"
	"github.com/sandgorgon/tui/term"
)

// readUntil reads from r in the background until its accumulated
// output contains want or timeout elapses, whichever comes first.
func readUntil(t *testing.T, r io.Reader, want string, timeout time.Duration) string {
	t.Helper()
	chunks := make(chan []byte, 16)
	go func() {
		buf := make([]byte, 256)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				b := make([]byte, n)
				copy(b, buf[:n])
				chunks <- b
			}
			if err != nil {
				close(chunks)
				return
			}
		}
	}()

	var got []byte
	deadline := time.After(timeout)
	for {
		select {
		case b, ok := <-chunks:
			if !ok {
				return string(got)
			}
			got = append(got, b...)
			if strings.Contains(string(got), want) {
				return string(got)
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %q in output; got so far: %q", want, got)
			return string(got)
		}
	}
}

func TestOpenLoopback(t *testing.T) {
	master, slave, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	defer slave.Close()

	if !term.IsTerminal(master) {
		t.Error("IsTerminal(master) = false, want true")
	}
	if !term.IsTerminal(slave) {
		t.Error("IsTerminal(slave) = false, want true")
	}

	// Raw mode on the slave: without it, the default canonical mode
	// would buffer by line (our test bytes have no trailing newline)
	// and echo would duplicate bytes back out through the master.
	if _, err := term.MakeRaw(slave); err != nil {
		t.Fatalf("MakeRaw(slave): %v", err)
	}

	if _, err := master.Write([]byte("ping")); err != nil {
		t.Fatalf("master.Write: %v", err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(slave, buf); err != nil {
		t.Fatalf("slave read: %v", err)
	}
	if string(buf) != "ping" {
		t.Errorf("slave got %q, want %q", buf, "ping")
	}

	if _, err := slave.Write([]byte("pong")); err != nil {
		t.Fatalf("slave.Write: %v", err)
	}
	if _, err := io.ReadFull(master, buf); err != nil {
		t.Fatalf("master read: %v", err)
	}
	if string(buf) != "pong" {
		t.Errorf("master got %q, want %q", buf, "pong")
	}
}

func TestStartAndResize(t *testing.T) {
	// The child sleeps briefly first so Resize (called after Start
	// returns) is guaranteed to land before stty actually checks the
	// size — Start only guarantees the process has been launched, not
	// that it's reached this point yet.
	cmd := exec.Command("sh", "-c", "sleep 0.2; stty size")
	p, err := pty.Start(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	if err := p.Resize(term.Size{Rows: 24, Cols: 96}); err != nil {
		t.Fatalf("Resize: %v", err)
	}

	got := readUntil(t, p, "24 96", 3*time.Second)
	if err := cmd.Wait(); err != nil {
		t.Errorf("Wait: %v", err)
	}
	if !strings.Contains(got, "24 96") {
		t.Errorf("stty size output = %q, want it to contain %q", got, "24 96")
	}
}

// TestCtrlCTriggersSIGINTViaLineDiscipline is the key empirical claim
// behind package pty not implementing manual SIGINT/SIGTSTP forwarding
// (see the Signal doc comment): a freshly allocated pty slave starts in
// cooked/ISIG-enabled mode by default, independent of the host
// terminal's own mode, so simply writing the raw interrupt byte to the
// master is enough for the pty's own line discipline to deliver a real
// SIGINT to the child — no explicit signal send required.
func TestCtrlCTriggersSIGINTViaLineDiscipline(t *testing.T) {
	cmd := exec.Command("sh", "-c", `trap 'echo GOT_INT; exit 0' INT; sleep 5`)
	p, err := pty.Start(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	time.Sleep(150 * time.Millisecond) // let the trap get installed
	if _, err := p.Write([]byte{0x03}); err != nil {
		t.Fatalf("write Ctrl-C byte: %v", err)
	}

	got := readUntil(t, p, "GOT_INT", 3*time.Second)
	if err := cmd.Wait(); err != nil {
		t.Errorf("Wait: %v", err)
	}
	if !strings.Contains(got, "GOT_INT") {
		t.Errorf("output = %q, want it to contain %q (child's SIGINT trap)", got, "GOT_INT")
	}
}
