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

// overlayScopeWidget extends scopeWidget with OverlayBounds and
// OutsideClicker, for testing App.HandleInput's outside-click handling
// without needing a real Modal/CommandPalette.
type overlayScopeWidget struct {
	scopeWidget
	bounds       cell.Rect
	boundsSet    bool
	outsideCalls []input.MouseEvent
	outsideCmd   Cmd
}

func (w *overlayScopeWidget) OverlayBounds() (cell.Rect, bool) { return w.bounds, w.boundsSet }
func (w *overlayScopeWidget) HandleOutsideClick(me input.MouseEvent) Cmd {
	w.outsideCalls = append(w.outsideCalls, me)
	return w.outsideCmd
}

func TestHandleInputWithholdsClickOutsideOverlayBounds(t *testing.T) {
	inner := &fakeWidget{focusable: true}
	scope := &overlayScopeWidget{
		scopeWidget: scopeWidget{active: true, inner: []Widget{inner}},
		bounds:      cell.Rect{X: 2, Y: 2, W: 4, H: 4},
		boundsSet:   true,
	}
	m := &widgetHostModel{node: Component("modal", nil, func() Widget { return scope })}
	app := NewApp(m, 10, 10)

	app.HandleInput(input.MouseEvent{X: 0, Y: 0, Button: input.MouseLeft})
	if len(inner.events) != 0 {
		t.Errorf("expected a click outside overlay bounds to be withheld from the scope's focused widget, got events=%v", inner.events)
	}
}

func TestHandleInputDeliversClickInsideOverlayBounds(t *testing.T) {
	inner := &fakeWidget{focusable: true}
	scope := &overlayScopeWidget{
		scopeWidget: scopeWidget{active: true, inner: []Widget{inner}},
		bounds:      cell.Rect{X: 2, Y: 2, W: 4, H: 4},
		boundsSet:   true,
	}
	m := &widgetHostModel{node: Component("modal", nil, func() Widget { return scope })}
	app := NewApp(m, 10, 10)

	app.HandleInput(input.MouseEvent{X: 3, Y: 3, Button: input.MouseLeft})
	if len(inner.events) != 1 {
		t.Errorf("expected a click inside overlay bounds to still reach the scope's focused widget, got events=%v", inner.events)
	}
}

func TestHandleInputCallsOutsideClickerAndSkipsFocusedWidget(t *testing.T) {
	inner := &fakeWidget{focusable: true}
	outsideMsg := Msg("outside-clicked")
	scope := &overlayScopeWidget{
		scopeWidget: scopeWidget{active: true, inner: []Widget{inner}},
		bounds:      cell.Rect{X: 2, Y: 2, W: 4, H: 4},
		boundsSet:   true,
		outsideCmd:  func() Msg { return outsideMsg },
	}
	m := &widgetHostModel{node: Component("modal", nil, func() Widget { return scope })}
	app := NewApp(m, 10, 10)

	cmds := app.HandleInput(input.MouseEvent{X: 0, Y: 0, Button: input.MouseLeft})
	if len(scope.outsideCalls) != 1 {
		t.Fatalf("expected HandleOutsideClick to be called once, got %d", len(scope.outsideCalls))
	}
	if len(inner.events) != 0 {
		t.Errorf("expected inner widget not to receive the outside click, got events=%v", inner.events)
	}
	found := false
	for _, c := range cmds {
		if c() == outsideMsg {
			found = true
		}
	}
	if !found {
		t.Errorf("expected HandleOutsideClick's Cmd to be included in the returned cmds, got %v", cmds)
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
