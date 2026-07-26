package tui

import (
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
)

func TestFocusableForwardsEventsThroughOnEvent(t *testing.T) {
	var got input.Event
	node := Focusable("f", Text("hi", cell.Style{}), func(e input.Event) Msg {
		got = e
		return "handled"
	})
	r := reconcile(nil, node)

	cmd := r.widget.HandleEvent(input.KeyEvent{Rune: 'x'})
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd when onEvent returns a Msg")
	}
	if msg := cmd(); msg != "handled" {
		t.Errorf("cmd() = %v, want %q", msg, "handled")
	}
	if got != (input.KeyEvent{Rune: 'x'}) {
		t.Errorf("onEvent received %v, want KeyEvent{Rune:'x'}", got)
	}
}

func TestFocusableNilOnEventProducesNoCmd(t *testing.T) {
	r := reconcile(nil, Focusable("f", Text("hi", cell.Style{}), nil))
	if cmd := r.widget.HandleEvent(input.KeyEvent{Rune: 'x'}); cmd != nil {
		t.Error("expected nil Cmd when onEvent is nil")
	}
}

func TestFocusableIsFocusableAndTracksSetFocused(t *testing.T) {
	r := reconcile(nil, Focusable("f", Text("hi", cell.Style{}), nil))
	if !r.widget.Focusable() {
		t.Fatal("Focusable's widget should report Focusable() == true")
	}
	fw := r.widget.(*focusableWidget)
	if fw.focused {
		t.Fatal("should not be focused before SetFocused is called")
	}
	r.widget.SetFocused(true)
	if !fw.focused {
		t.Error("expected focused == true after SetFocused(true)")
	}
}

func TestListRetainsScrollOffsetAcrossReconcile(t *testing.T) {
	items := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}
	r := reconcile(nil, List(items, 9, cell.Style{}, cell.Style{}, nil))

	buf := cell.NewBuffer(10, 3) // shorter than the list, forces scrolling
	r.widget.Paint(cell.NewPainter(buf))

	lw := r.widget.(*listWidget)
	if lw.scrollOffset == 0 {
		t.Fatal("expected scrollOffset to advance so the selected (last) row stays visible")
	}
	offset := lw.scrollOffset

	// A fresh items slice (as Model.View() would build every frame) with
	// the same selection must not reset the retained scroll position.
	r = reconcile(r, List(append([]string(nil), items...), 9, cell.Style{}, cell.Style{}, nil))
	if got := r.widget.(*listWidget).scrollOffset; got != offset {
		t.Errorf("scrollOffset after reconcile = %d, want %d (retained state must survive a Model-driven re-render)", got, offset)
	}
}

func TestListForwardsEventsThroughOnEvent(t *testing.T) {
	var got input.Event
	node := List([]string{"a", "b"}, 0, cell.Style{}, cell.Style{}, func(e input.Event) Msg {
		got = e
		return "moved"
	})
	r := reconcile(nil, node)

	cmd := r.widget.HandleEvent(input.KeyEvent{Key: input.KeyDown})
	if cmd == nil || cmd() != "moved" {
		t.Fatal("expected onEvent's Msg to come back through HandleEvent's Cmd")
	}
	if got != (input.KeyEvent{Key: input.KeyDown}) {
		t.Errorf("onEvent received %v", got)
	}
}
