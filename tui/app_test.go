package tui

import (
	"fmt"
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/layout"
)

type counterModel struct {
	count int
}

func (m *counterModel) Init() Cmd { return nil }

func (m *counterModel) Update(msg Msg) (Model, Cmd) {
	if s, ok := msg.(string); ok && s == "inc" {
		m.count++
	}
	return m, nil
}

func (m *counterModel) View() Node {
	return Text(fmt.Sprintf("count=%d", m.count), cell.Style{})
}

func TestAppDispatchUpdatesModelAndRepaints(t *testing.T) {
	a := NewApp(&counterModel{}, 12, 1)
	if got := a.Buffer().String(); got != "count=0     " {
		t.Fatalf("initial Buffer = %q", got)
	}

	a.Dispatch("inc")
	a.Dispatch("inc")

	if got := a.Buffer().String(); got != "count=2     " {
		t.Errorf("Buffer after two \"inc\" = %q, want %q", got, "count=2     ")
	}
}

func TestAppDispatchPassesClipboardMsgThroughToModelUnchanged(t *testing.T) {
	// Dispatch is I/O-free by design (docs/DESIGN.md §10) — only Run's
	// own goroutine ever actually writes a ClipboardMsg (see
	// CopyToClipboard's doc comment); Dispatch itself must treat it
	// like any other Msg, not intercept or drop it.
	var seen Msg
	m := &appTestModel{onUpdate: func(msg Msg) { seen = msg }}
	a := NewApp(m, 10, 3)

	a.Dispatch(ClipboardMsg{Text: "copied text"})

	cm, ok := seen.(ClipboardMsg)
	if !ok || cm.Text != "copied text" {
		t.Errorf("Model.Update saw %#v, want ClipboardMsg{Text: \"copied text\"}", seen)
	}
}

func TestAppResizeRepaintsAtNewSize(t *testing.T) {
	a := NewApp(&counterModel{}, 12, 1)
	a.Resize(8, 1)
	if got := a.Buffer().String(); got != "count=0 " {
		t.Errorf("Buffer after Resize = %q, want %q", got, "count=0 ")
	}
}

// focusTestModel has two Focusable regions, each forwarding its own
// tagged Msg through onEvent, so focus routing can be checked
// end-to-end: which widget receives an event, and that the raw event
// also reaches Update independently.
type focusTestModel struct {
	seen []Msg
}

func (m *focusTestModel) Init() Cmd { return nil }

func (m *focusTestModel) Update(msg Msg) (Model, Cmd) {
	m.seen = append(m.seen, msg)
	return m, nil
}

func (m *focusTestModel) View() Node {
	return Box(layout.Horizontal,
		Child(layout.Fill(1), Focusable("left", Text("L", cell.Style{}), func(e input.Event) Msg { return "left" })),
		Child(layout.Fill(1), Focusable("right", Text("R", cell.Style{}), func(e input.Event) Msg { return "right" })),
	)
}

func TestAppHandleInputRoutesToFocusedWidgetAndModel(t *testing.T) {
	m := &focusTestModel{}
	a := NewApp(m, 10, 3)

	if len(a.focusables) != 2 {
		t.Fatalf("focusables = %d, want 2", len(a.focusables))
	}
	if a.focusIdx != 0 {
		t.Fatalf("initial focusIdx = %d, want 0", a.focusIdx)
	}

	cmds := a.HandleInput(input.KeyEvent{Rune: 'x'})
	if len(cmds) != 1 {
		t.Fatalf("cmds = %d, want 1 (only the focused widget's onEvent produced one)", len(cmds))
	}
	if got := cmds[0](); got != "left" {
		t.Errorf("cmds[0]() = %v, want %q (focus starts on \"left\")", got, "left")
	}

	a.HandleInput(input.KeyEvent{Key: input.KeyTab})
	if a.focusIdx != 1 {
		t.Fatalf("focusIdx after Tab = %d, want 1", a.focusIdx)
	}

	cmds = a.HandleInput(input.KeyEvent{Rune: 'y'})
	if got := cmds[0](); got != "right" {
		t.Errorf("cmds[0]() after Tab = %v, want %q", got, "right")
	}

	a.HandleInput(input.KeyEvent{Key: input.KeyTab, Mod: input.ModShift})
	if a.focusIdx != 0 {
		t.Fatalf("focusIdx after Shift+Tab = %d, want 0", a.focusIdx)
	}

	// Tab/Shift-Tab must never reach Update; the two Rune presses must.
	if len(m.seen) != 2 {
		t.Fatalf("Model.Update saw %d msgs, want 2 (Tab/Shift-Tab excluded): %v", len(m.seen), m.seen)
	}
	if ke, ok := m.seen[0].(input.KeyEvent); !ok || ke.Rune != 'x' {
		t.Errorf("Update's first msg = %#v, want KeyEvent{Rune:'x'}", m.seen[0])
	}
}

func TestAppSetFocus(t *testing.T) {
	m := &focusTestModel{}
	a := NewApp(m, 10, 3)

	if got := a.FocusIndex(); got != 0 {
		t.Fatalf("initial FocusIndex = %d, want 0", got)
	}

	if ok := a.SetFocus(1); !ok {
		t.Fatal("SetFocus(1) = false, want true")
	}
	if got := a.FocusIndex(); got != 1 {
		t.Fatalf("FocusIndex after SetFocus(1) = %d, want 1", got)
	}

	cmds := a.HandleInput(input.KeyEvent{Rune: 'y'})
	if got := cmds[0](); got != "right" {
		t.Errorf("cmds[0]() after SetFocus(1) = %v, want %q", got, "right")
	}

	if ok := a.SetFocus(2); ok {
		t.Error("SetFocus(2) = true, want false (only 2 focusables)")
	}
	if ok := a.SetFocus(-1); ok {
		t.Error("SetFocus(-1) = true, want false")
	}
	if got := a.FocusIndex(); got != 1 {
		t.Errorf("FocusIndex after out-of-range SetFocus = %d, want unchanged 1", got)
	}
}

func TestAppSetFocusWithNoFocusablesIsNoop(t *testing.T) {
	a := NewApp(&counterModel{}, 10, 1)
	if ok := a.SetFocus(0); ok {
		t.Error("SetFocus(0) with no focusables = true, want false")
	}
	if got := a.FocusIndex(); got != 0 {
		t.Errorf("FocusIndex = %d, want 0", got)
	}
}

func TestAppTabWithNoFocusablesIsNoop(t *testing.T) {
	a := NewApp(&counterModel{}, 10, 1)
	if cmds := a.HandleInput(input.KeyEvent{Key: input.KeyTab}); cmds != nil {
		t.Errorf("Tab with no focusables should produce no cmds, got %v", cmds)
	}
	if a.focusIdx != 0 {
		t.Errorf("focusIdx = %d, want 0", a.focusIdx)
	}
}

// focusAwareTestModel implements FocusAware alongside the same
// two-Focusable ("left"/"right") shape as focusTestModel, recording
// every SetFocusedKey call so tests can assert on the sequence.
type focusAwareTestModel struct {
	lastKey    any
	keyHistory []any
}

func (m *focusAwareTestModel) Init() Cmd                   { return nil }
func (m *focusAwareTestModel) Update(msg Msg) (Model, Cmd) { return m, nil }
func (m *focusAwareTestModel) SetFocusedKey(key any) {
	m.lastKey = key
	m.keyHistory = append(m.keyHistory, key)
}
func (m *focusAwareTestModel) View() Node {
	return Box(layout.Horizontal,
		Child(layout.Fill(1), Focusable("left", Text("L", cell.Style{}), func(e input.Event) Msg { return nil })),
		Child(layout.Fill(1), Focusable("right", Text("R", cell.Style{}), func(e input.Event) Msg { return nil })),
	)
}

func TestAppReportsFocusedKeyToFocusAwareModel(t *testing.T) {
	m := &focusAwareTestModel{}
	a := NewApp(m, 10, 3)

	// No previous frame exists yet at construction, so the very first
	// call must report nil rather than guessing at focusIdx 0's key.
	if len(m.keyHistory) == 0 || m.keyHistory[0] != nil {
		t.Fatalf("first SetFocusedKey call(s) = %v, want first == nil", m.keyHistory)
	}

	a.HandleInput(input.KeyEvent{Key: input.KeyTab})
	if got := m.lastKey; got != "right" {
		t.Errorf("SetFocusedKey after Tab = %v, want %q", got, "right")
	}

	a.HandleInput(input.KeyEvent{Key: input.KeyTab, Mod: input.ModShift})
	if got := m.lastKey; got != "left" {
		t.Errorf("SetFocusedKey after Shift+Tab = %v, want %q", got, "left")
	}

	if ok := a.SetFocus(1); !ok {
		t.Fatal("SetFocus(1) failed")
	}
	if got := m.lastKey; got != "right" {
		t.Errorf("SetFocusedKey after SetFocus(1) = %v, want %q", got, "right")
	}
}

// focusAwareUnkeyedModel is FocusAware over a single Focusable that
// carries no explicit Node.Key (positional matching only), to confirm
// an unkeyed widget reports key == nil rather than some placeholder.
type focusAwareUnkeyedModel struct{ lastKey any }

func (m *focusAwareUnkeyedModel) Init() Cmd                   { return nil }
func (m *focusAwareUnkeyedModel) Update(msg Msg) (Model, Cmd) { return m, nil }
func (m *focusAwareUnkeyedModel) SetFocusedKey(key any)       { m.lastKey = key }
func (m *focusAwareUnkeyedModel) View() Node {
	return Focusable(nil, Text("X", cell.Style{}), func(e input.Event) Msg { return nil })
}

func TestAppReportsNilKeyForUnkeyedFocusable(t *testing.T) {
	m := &focusAwareUnkeyedModel{}
	a := NewApp(m, 10, 1)

	// One more render (a no-op Tab, single focusable so it's a no-op
	// cycle back to itself) to get past the always-nil first frame and
	// confirm a real, unkeyed focused widget still reports nil.
	a.HandleInput(input.KeyEvent{Key: input.KeyTab})
	if m.lastKey != nil {
		t.Errorf("SetFocusedKey for an unkeyed Focusable = %v, want nil", m.lastKey)
	}
}

func TestAppInitCmdConsumedOnce(t *testing.T) {
	a := NewApp(&counterModel{}, 10, 1)
	if a.InitCmd() != nil {
		t.Fatal("counterModel.Init returns nil, so InitCmd() should be nil")
	}

	ranTimes := 0
	m := &initCmdModel{ran: &ranTimes}
	a2 := NewApp(m, 10, 1)
	c := a2.InitCmd()
	if c == nil {
		t.Fatal("expected a non-nil Cmd from InitCmd the first time")
	}
	if a2.InitCmd() != nil {
		t.Error("InitCmd should return nil on a second call (consumed)")
	}
	c()
	if ranTimes != 1 {
		t.Errorf("ran = %d, want 1", ranTimes)
	}
}

type initCmdModel struct{ ran *int }

func (m *initCmdModel) Init() Cmd {
	return func() Msg { *m.ran++; return nil }
}
func (m *initCmdModel) Update(msg Msg) (Model, Cmd) { return m, nil }
func (m *initCmdModel) View() Node                  { return Text("", cell.Style{}) }

// TestResolveWidgetCmdAppliesSynchronously is the regression test for
// #18: a widget-sourced Cmd (the shape TextArea.OnCursorChange's
// callback produces) must land in Model.Update before
// resolveWidgetCmd returns, with no async goroutine/channel hop in
// between — that hop had no ordering guarantee against a caller's own
// later, synchronous Dispatch call, which is exactly the race #18
// reported (a jump to another card observing a stale cursor offset).
func TestResolveWidgetCmdAppliesSynchronously(t *testing.T) {
	m := &focusTestModel{}
	a := NewApp(m, 10, 3)
	m.seen = nil // drop the initial View-triggered noise, if any

	cmd := func() Msg { return "cursor-moved" }
	if follow := a.resolveWidgetCmd(cmd); follow != nil {
		t.Fatalf("resolveWidgetCmd follow-up = %v, want nil (focusTestModel.Update returns no Cmd)", follow)
	}

	if len(m.seen) != 1 || m.seen[0] != "cursor-moved" {
		t.Fatalf("Model.Update saw %v, want [\"cursor-moved\"] applied synchronously", m.seen)
	}
}

// TestResolveWidgetCmdAppliesBatchInOrder covers the shape
// TextArea.HandleEvent actually returns when both OnChange and
// OnCursorChange fire for the same edit (tui.Batch of two Cmds) — each
// sub-Cmd's Msg must reach Update, in order, all before
// resolveWidgetCmd returns.
func TestResolveWidgetCmdAppliesBatchInOrder(t *testing.T) {
	m := &focusTestModel{}
	a := NewApp(m, 10, 3)
	m.seen = nil

	batch := Batch(
		func() Msg { return "changed:a" },
		func() Msg { return "cursor:1" },
	)
	a.resolveWidgetCmd(batch)

	want := []Msg{"changed:a", "cursor:1"}
	if len(m.seen) != len(want) || m.seen[0] != want[0] || m.seen[1] != want[1] {
		t.Fatalf("Model.Update saw %v, want %v in order", m.seen, want)
	}
}

// TestResolveWidgetCmdLeavesSpecialMsgsUnresolved documents the one
// deliberate exception: QuitMsg/ClipboardMsg/FocusMsg aren't fed
// through Dispatch here (Dispatch is I/O-free by design and can't stop
// Run or write the clipboard) — they come back as a Cmd for Run's
// normal async path to handle exactly as before.
func TestResolveWidgetCmdLeavesSpecialMsgsUnresolved(t *testing.T) {
	m := &focusTestModel{}
	a := NewApp(m, 10, 3)
	m.seen = nil

	cmd := func() Msg { return ClipboardMsg{Text: "hi"} }
	follow := a.resolveWidgetCmd(cmd)

	if len(m.seen) != 0 {
		t.Fatalf("Model.Update saw %v, want none — ClipboardMsg must not reach Update via resolveWidgetCmd", m.seen)
	}
	if len(follow) != 1 {
		t.Fatalf("follow-up cmds = %d, want 1 (the ClipboardMsg, for Run to handle)", len(follow))
	}
	cm, ok := follow[0]().(ClipboardMsg)
	if !ok || cm.Text != "hi" {
		t.Fatalf("follow-up cmd produced %#v, want ClipboardMsg{Text: \"hi\"}", follow[0]())
	}
}
