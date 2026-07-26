package tui

import "testing"

func TestQuit(t *testing.T) {
	msg := Quit()()
	if _, ok := msg.(QuitMsg); !ok {
		t.Errorf("Quit()() = %#v, want QuitMsg", msg)
	}
}

func TestCopyToClipboard(t *testing.T) {
	msg := CopyToClipboard("hello")()
	cm, ok := msg.(ClipboardMsg)
	if !ok {
		t.Fatalf("CopyToClipboard(...)() = %#v (%T), want ClipboardMsg", msg, msg)
	}
	if cm.Text != "hello" {
		t.Errorf("ClipboardMsg.Text = %q, want %q", cm.Text, "hello")
	}
}

func TestBatchEmpty(t *testing.T) {
	if Batch() != nil {
		t.Error("Batch() with no cmds should be nil")
	}
	if Batch(nil, nil) != nil {
		t.Error("Batch(nil, nil) should be nil")
	}
}

func TestBatchRunsEachCmd(t *testing.T) {
	c1 := func() Msg { return "a" }
	c2 := func() Msg { return "b" }
	batched := Batch(c1, nil, c2)
	if batched == nil {
		t.Fatal("Batch with at least one non-nil Cmd should be non-nil")
	}

	msg := batched()
	bm, ok := msg.(BatchMsg)
	if !ok {
		t.Fatalf("Batch()() = %#v (%T), want BatchMsg", msg, msg)
	}
	if len(bm) != 2 {
		t.Fatalf("BatchMsg len = %d, want 2 (nil filtered out)", len(bm))
	}
	if got := bm[0](); got != "a" {
		t.Errorf("bm[0]() = %v, want %q", got, "a")
	}
	if got := bm[1](); got != "b" {
		t.Errorf("bm[1]() = %v, want %q", got, "b")
	}
}
