package tui

import (
	"testing"

	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/layout"
)

type requestFocusMsg struct{ idx int }

// focusReqModel implements FocusRequester the way the contract asks:
// it reports a request only from the Update that made it, clearing it
// at the start of the next.
type focusReqModel struct {
	node Node
	req  int // requested index + 1; 0 = none
}

func (m *focusReqModel) Init() Cmd { return nil }
func (m *focusReqModel) Update(msg Msg) (Model, Cmd) {
	m.req = 0
	switch v := msg.(type) {
	case requestFocusMsg:
		m.req = v.idx + 1
	case ReleaseMsg:
		m.req = 2 + 1 // always send a release to the third widget
	}
	return m, nil
}
func (m *focusReqModel) View() Node                  { return m.node }
func (m *focusReqModel) RequestedFocus() (int, bool) { return m.req - 1, m.req > 0 }

func newFocusReqApp() (*App, *focusReqModel, *rawKeyWidget, *fakeWidget, *fakeWidget) {
	claimer := &rawKeyWidget{fakeWidget: fakeWidget{focusable: true}, release: input.KeyEvent{Rune: '\\', Mod: input.ModCtrl}}
	middle := &fakeWidget{focusable: true}
	target := &fakeWidget{focusable: true}
	m := &focusReqModel{node: Box(layout.Vertical,
		Child(layout.Fill(1), Component("claimer", nil, func() Widget { return claimer })),
		Child(layout.Fill(1), Component("middle", nil, func() Widget { return middle })),
		Child(layout.Fill(1), Component("target", nil, func() Widget { return target })),
	)}
	return NewApp(m, 10, 9), m, claimer, middle, target
}

func TestFocusRequesterMovesFocusSynchronouslyInDispatch(t *testing.T) {
	app, _, _, _, target := newFocusReqApp()

	if cmd := app.Dispatch(requestFocusMsg{idx: 2}); cmd != nil {
		t.Fatalf("Dispatch returned a Cmd (%v); the request must not need one", cmd)
	}
	if app.FocusIndex() != 2 || !target.focused {
		t.Fatalf("FocusIndex = %d, target.focused = %v, want focus on widget 2 right after Dispatch", app.FocusIndex(), target.focused)
	}
}

func TestFocusRequestIsOneShot(t *testing.T) {
	app, _, _, _, _ := newFocusReqApp()
	app.Dispatch(requestFocusMsg{idx: 2})
	app.SetFocus(1) // the user moves focus elsewhere

	app.Dispatch(struct{}{}) // an unrelated Update, e.g. a redraw tick
	if app.FocusIndex() != 1 {
		t.Fatalf("FocusIndex = %d after an unrelated Dispatch, want 1 (a stale request must not refocus)", app.FocusIndex())
	}
}

func TestFocusRequestOutOfRangeIsIgnored(t *testing.T) {
	app, _, _, _, _ := newFocusReqApp()
	app.SetFocus(1)
	app.Dispatch(requestFocusMsg{idx: 99})
	app.Dispatch(requestFocusMsg{idx: -1})
	if app.FocusIndex() != 1 {
		t.Fatalf("FocusIndex = %d, want 1 (out-of-range requests are ignored)", app.FocusIndex())
	}
}

// TestFocusRequestBeatsTypeAhead is the reason FocusRequester exists: a
// key that arrives right behind the release key must reach the widget
// the release sent focus to, not whichever had focus a moment earlier.
// HandleInput is called back to back with no Cmd run in between, as a
// burst of input (a paste, a key macro) would be.
func TestFocusRequestBeatsTypeAhead(t *testing.T) {
	app, _, claimer, middle, target := newFocusReqApp()

	app.HandleInput(input.KeyEvent{Rune: '\\', Mod: input.ModCtrl}) // release -> requested focus on widget 2
	app.HandleInput(input.KeyEvent{Rune: 'x'})

	if len(claimer.events) != 0 || len(middle.events) != 0 {
		t.Errorf("type-ahead reached the wrong widget: claimer=%v middle=%v", claimer.events, middle.events)
	}
	if len(target.events) != 1 || target.events[0] != (input.KeyEvent{Rune: 'x'}) {
		t.Errorf("target.events = %v, want the 'x' typed right after the release", target.events)
	}
}
