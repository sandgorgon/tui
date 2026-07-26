package tui

import (
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/layout"
)

// scopeWidget is a minimal FocusScope test double: active toggles
// whether it claims focus, and inner is what it exposes as its own
// Focusables while active.
type scopeWidget struct {
	fakeWidget
	active  bool
	inner   []Widget
	painted bool
}

func (w *scopeWidget) Active() bool               { return w.active }
func (w *scopeWidget) Focusables() []Widget       { return w.inner }
func (w *scopeWidget) PaintOverlay(*cell.Painter) { w.painted = true }

func TestCollectFocusablesIgnoresInactiveScope(t *testing.T) {
	bg := &fakeWidget{focusable: true}
	scope := &scopeWidget{active: false, inner: []Widget{&fakeWidget{focusable: true}}}

	tree := Box(layout.Horizontal,
		Child(layout.Fill(1), Component("bg", nil, func() Widget { return bg })),
		Child(layout.Fill(1), Component("modal", nil, func() Widget { return scope })),
	)
	r := reconcile(nil, tree)

	got := collectFocusables(r)
	if len(got) != 1 || got[0] != Widget(bg) {
		t.Errorf("collectFocusables = %v, want just the background widget (scope inactive)", got)
	}
}

func TestCollectFocusablesUsesActiveScopeExclusively(t *testing.T) {
	bg := &fakeWidget{focusable: true}
	inner1 := &fakeWidget{focusable: true}
	inner2 := &fakeWidget{focusable: true}
	scope := &scopeWidget{active: true, inner: []Widget{inner1, inner2}}

	tree := Box(layout.Horizontal,
		Child(layout.Fill(1), Component("bg", nil, func() Widget { return bg })),
		Child(layout.Fill(1), Component("modal", nil, func() Widget { return scope })),
	)
	r := reconcile(nil, tree)

	got := collectFocusables(r)
	if len(got) != 2 || got[0] != Widget(inner1) || got[1] != Widget(inner2) {
		t.Errorf("collectFocusables = %v, want exactly the active scope's own Focusables, background excluded", got)
	}
}

func TestAppPaintsOverlayWhenScopeActive(t *testing.T) {
	scope := &scopeWidget{active: true}
	m := &widgetHostModel{node: Component("modal", nil, func() Widget { return scope })}
	NewApp(m, 10, 5)

	if !scope.painted {
		t.Error("expected PaintOverlay to be called while the scope is active")
	}
}

func TestAppDoesNotPaintOverlayWhenScopeInactive(t *testing.T) {
	scope := &scopeWidget{active: false}
	m := &widgetHostModel{node: Component("modal", nil, func() Widget { return scope })}
	NewApp(m, 10, 5)

	if scope.painted {
		t.Error("PaintOverlay should not be called while the scope is inactive")
	}
}

func TestAppTabStaysWithinActiveScope(t *testing.T) {
	inner1 := &fakeWidget{focusable: true}
	inner2 := &fakeWidget{focusable: true}
	scope := &scopeWidget{active: true, inner: []Widget{inner1, inner2}}
	bg := &fakeWidget{focusable: true}

	m := &widgetHostModel{node: Box(layout.Horizontal,
		Child(layout.Fill(1), Component("bg", nil, func() Widget { return bg })),
		Child(layout.Fill(1), Component("modal", nil, func() Widget { return scope })),
	)}
	app := NewApp(m, 10, 5)

	app.HandleInput(input.KeyEvent{Key: input.KeyTab})
	if inner1.focused || !inner2.focused {
		t.Errorf("after Tab, want inner2 focused: inner1.focused=%v inner2.focused=%v", inner1.focused, inner2.focused)
	}
	if bg.focused {
		t.Error("background widget should never receive focus while the scope is active")
	}
}
