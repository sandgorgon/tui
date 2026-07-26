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

func TestAppTabWithNoFocusablesIsNoop(t *testing.T) {
	a := NewApp(&counterModel{}, 10, 1)
	if cmds := a.HandleInput(input.KeyEvent{Key: input.KeyTab}); cmds != nil {
		t.Errorf("Tab with no focusables should produce no cmds, got %v", cmds)
	}
	if a.focusIdx != 0 {
		t.Errorf("focusIdx = %d, want 0", a.focusIdx)
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
