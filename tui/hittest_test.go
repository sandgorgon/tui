package tui

import (
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/layout"
)

func TestCollectRectsRecordsAbsoluteBounds(t *testing.T) {
	a := &fakeWidget{focusable: true}
	b := &fakeWidget{focusable: true}
	tree := Box(layout.Horizontal,
		Child(layout.Length(4), Component("a", nil, func() Widget { return a })),
		Child(layout.Fill(1), Component("b", nil, func() Widget { return b })),
	)
	r := reconcile(nil, tree)

	rects := map[Widget]cell.Rect{}
	collectRects(r, cell.Rect{X: 0, Y: 0, W: 10, H: 3}, rects)

	if got, want := rects[Widget(a)], (cell.Rect{X: 0, Y: 0, W: 4, H: 3}); got != want {
		t.Errorf("rects[a] = %+v, want %+v", got, want)
	}
	if got, want := rects[Widget(b)], (cell.Rect{X: 4, Y: 0, W: 6, H: 3}); got != want {
		t.Errorf("rects[b] = %+v, want %+v", got, want)
	}
}

func TestCollectRectsNestedBoxesAccumulateOffset(t *testing.T) {
	inner := &fakeWidget{focusable: true}
	tree := Box(layout.Vertical,
		Child(layout.Length(2), Text("header", cell.Style{})),
		Child(layout.Fill(1), Box(layout.Horizontal,
			Child(layout.Length(3), Text("side", cell.Style{})),
			Child(layout.Fill(1), Component("inner", nil, func() Widget { return inner })),
		)),
	)
	r := reconcile(nil, tree)

	rects := map[Widget]cell.Rect{}
	collectRects(r, cell.Rect{X: 0, Y: 0, W: 10, H: 8}, rects)

	// header takes rows 0-1, the horizontal split starts at row 2; side
	// takes cols 0-2, inner starts at col 3.
	if got, want := rects[Widget(inner)], (cell.Rect{X: 3, Y: 2, W: 7, H: 6}); got != want {
		t.Errorf("rects[inner] = %+v, want %+v", got, want)
	}
}

func TestAppMouseClickMovesFocusAndTranslatesCoordinates(t *testing.T) {
	var got input.Event
	m := &widgetHostModel{node: Box(layout.Horizontal,
		Child(layout.Length(5), Focusable("left", Text("L", cell.Style{}), func(e input.Event) Msg { return "left" })),
		Child(layout.Fill(1), Focusable("right", Text("R", cell.Style{}), func(e input.Event) Msg {
			got = e
			return "right"
		})),
	)}
	app := NewApp(m, 15, 4)
	if app.focusIdx != 0 {
		t.Fatalf("initial focusIdx = %d, want 0", app.focusIdx)
	}

	// (7,1) is inside the right pane (which starts at absolute x=5),
	// so the widget should see it translated to local (2,1).
	cmds := app.HandleInput(input.MouseEvent{X: 7, Y: 1, Button: input.MouseLeft})

	if app.focusIdx != 1 {
		t.Fatalf("focusIdx after click = %d, want 1 (click-to-focus)", app.focusIdx)
	}
	if len(cmds) != 1 || cmds[0]() != "right" {
		t.Fatalf("expected the clicked widget's onEvent Msg, got cmds=%v", cmds)
	}
	if got != (input.MouseEvent{X: 2, Y: 1, Button: input.MouseLeft}) {
		t.Errorf("onEvent received %v, want translated-to-local (2,1)", got)
	}
}

func TestAppMouseClickOnAlreadyFocusedWidgetDoesNotResetFocus(t *testing.T) {
	m := &widgetHostModel{node: Focusable("only", Text("x", cell.Style{}), func(e input.Event) Msg { return "hit" })}
	app := NewApp(m, 10, 3)

	cmds := app.HandleInput(input.MouseEvent{X: 1, Y: 1, Button: input.MouseLeft})
	if len(cmds) != 1 || cmds[0]() != "hit" {
		t.Fatalf("expected a click on the sole focusable to reach it, got cmds=%v", cmds)
	}
}

func TestAppMouseClickOutsideAnyWidgetStillDispatchesButDoesNotForward(t *testing.T) {
	var events []Msg
	m := &appTestModel{onUpdate: func(msg Msg) { events = append(events, msg) }}
	app := NewApp(m, 10, 3)

	// No focusables at all in this Model — click should just Dispatch,
	// no HandleEvent call to forward to (nothing to forward to).
	app.HandleInput(input.MouseEvent{X: 5, Y: 1, Button: input.MouseLeft})
	if len(events) != 1 {
		t.Fatalf("Model.Update should still see the raw event, got %d events", len(events))
	}
}

// appTestModel is a minimal Model with no focusable content, for tests
// that only care about whether Dispatch happened.
type appTestModel struct {
	onUpdate func(Msg)
}

func (m *appTestModel) Init() Cmd { return nil }
func (m *appTestModel) Update(msg Msg) (Model, Cmd) {
	if m.onUpdate != nil {
		m.onUpdate(msg)
	}
	return m, nil
}
func (m *appTestModel) View() Node { return Text("static", cell.Style{}) }
