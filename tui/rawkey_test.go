package tui

import (
	"testing"

	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/layout"
)

type rawKeyWidget struct {
	fakeWidget
	release input.KeyEvent
}

func (w *rawKeyWidget) WantsRawTab() bool          { return true }
func (w *rawKeyWidget) ReleaseKey() input.KeyEvent { return w.release }

func TestAppForwardsTabToWidgetClaimingRawTab(t *testing.T) {
	claimer := &rawKeyWidget{fakeWidget: fakeWidget{focusable: true}, release: input.KeyEvent{Key: input.KeyEsc}}
	m := &widgetHostModel{node: Component("claimer", nil, func() Widget { return claimer })}
	app := NewApp(m, 10, 5)

	app.HandleInput(input.KeyEvent{Key: input.KeyTab})

	if len(claimer.events) != 1 || claimer.events[0] != (input.KeyEvent{Key: input.KeyTab}) {
		t.Fatalf("expected the claiming widget to receive Tab directly, got events=%v", claimer.events)
	}
	if app.focusIdx != 0 {
		t.Error("focus should not have moved: the widget claimed Tab instead of releasing it")
	}
}

func TestAppReleaseKeyMovesFocusInsteadOfForwarding(t *testing.T) {
	claimer := &rawKeyWidget{fakeWidget: fakeWidget{focusable: true}, release: input.KeyEvent{Key: input.KeyEsc}}
	other := &fakeWidget{focusable: true}
	m := &widgetHostModel{node: Box(layout.Vertical,
		Child(layout.Fill(1), Component("claimer", nil, func() Widget { return claimer })),
		Child(layout.Fill(1), Component("other", nil, func() Widget { return other })),
	)}
	app := NewApp(m, 10, 5)

	app.HandleInput(input.KeyEvent{Key: input.KeyEsc})

	if len(claimer.events) != 0 {
		t.Errorf("release key should not have been forwarded to the claiming widget, got events=%v", claimer.events)
	}
	if app.focusIdx != 1 {
		t.Errorf("focusIdx = %d, want 1 (release key should move focus onward)", app.focusIdx)
	}
	if !other.focused {
		t.Error("expected the next widget to become focused after the release key")
	}
}

func TestAppTabStillNavigatesWhenNotClaimed(t *testing.T) {
	a := &fakeWidget{focusable: true}
	b := &fakeWidget{focusable: true}
	m := &widgetHostModel{node: Box(layout.Vertical,
		Child(layout.Fill(1), Component("a", nil, func() Widget { return a })),
		Child(layout.Fill(1), Component("b", nil, func() Widget { return b })),
	)}
	app := NewApp(m, 10, 5)

	app.HandleInput(input.KeyEvent{Key: input.KeyTab})

	if len(a.events) != 0 || len(b.events) != 0 {
		t.Errorf("Tab should not have been forwarded to either widget: a.events=%v b.events=%v", a.events, b.events)
	}
	if app.focusIdx != 1 {
		t.Errorf("focusIdx = %d, want 1 (ordinary Tab navigation)", app.focusIdx)
	}
}
