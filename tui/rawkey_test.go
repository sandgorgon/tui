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

// releaseModel records every Msg Update sees and, if onRelease is set,
// answers a ReleaseMsg with that Cmd.
type releaseModel struct {
	node      Node
	msgs      []Msg
	onRelease Cmd
}

func (m *releaseModel) Init() Cmd { return nil }
func (m *releaseModel) Update(msg Msg) (Model, Cmd) {
	m.msgs = append(m.msgs, msg)
	if _, ok := msg.(ReleaseMsg); ok {
		return m, m.onRelease
	}
	return m, nil
}
func (m *releaseModel) View() Node { return m.node }

func newReleaseApp(release input.KeyEvent, onRelease Cmd) (*App, *releaseModel, *rawKeyWidget, *fakeWidget) {
	claimer := &rawKeyWidget{fakeWidget: fakeWidget{focusable: true}, release: release}
	other := &fakeWidget{focusable: true}
	m := &releaseModel{onRelease: onRelease, node: Box(layout.Vertical,
		Child(layout.Fill(1), Component("claimer", nil, func() Widget { return claimer }).Key("c")),
		Child(layout.Fill(1), Component("other", nil, func() Widget { return other })),
	)}
	return NewApp(m, 10, 5), m, claimer, other
}

func TestAppReleaseKeyIsReportedToUpdateAsReleaseMsg(t *testing.T) {
	rel := input.KeyEvent{Rune: '\\', Mod: input.ModCtrl}
	app, m, claimer, other := newReleaseApp(rel, nil)

	app.HandleInput(rel)

	var got []ReleaseMsg
	for _, msg := range m.msgs {
		switch v := msg.(type) {
		case ReleaseMsg:
			got = append(got, v)
		case input.KeyEvent:
			t.Errorf("the release key must not reach Update as a raw KeyEvent, got %v", v)
		}
	}
	if len(got) != 1 {
		t.Fatalf("Update saw %d ReleaseMsg, want 1 (msgs=%v)", len(got), m.msgs)
	}
	if got[0].Key != rel || got[0].FromIndex != 0 || got[0].FromKey != "c" {
		t.Errorf("ReleaseMsg = %+v, want Key=%v FromIndex=0 FromKey=\"c\"", got[0], rel)
	}
	if len(claimer.events) != 0 {
		t.Errorf("release key must not be forwarded to the widget, got %v", claimer.events)
	}
	// The default is unchanged: with no override, focus moves onward.
	if app.focusIdx != 1 || !other.focused {
		t.Errorf("focusIdx = %d, other.focused = %v, want focus moved onward to 1", app.focusIdx, other.focused)
	}
}

func TestAppReleaseMsgCanOverrideWhereFocusLands(t *testing.T) {
	rel := input.KeyEvent{Rune: '\\', Mod: input.ModCtrl}
	app, _, _, _ := newReleaseApp(rel, SetFocusCmd(0))

	cmds := app.HandleInput(rel)

	var focus []FocusMsg
	for _, c := range cmds {
		if c == nil {
			continue
		}
		if fm, ok := c().(FocusMsg); ok {
			focus = append(focus, fm)
		}
	}
	if len(focus) != 1 || focus[0].Index != 0 {
		t.Fatalf("HandleInput returned focus msgs %v, want exactly FocusMsg{Index: 0} from Update", focus)
	}
	if !app.SetFocus(focus[0].Index) || app.focusIdx != 0 {
		t.Errorf("applying the override left focusIdx = %d, want 0", app.focusIdx)
	}
}
