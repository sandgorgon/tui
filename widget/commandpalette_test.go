package widget

import (
	"strings"
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/input"
	"github.com/sandgorgon/tui/layout"
	"github.com/sandgorgon/tui/style"
	"github.com/sandgorgon/tui/tui"
)

// handleAndRun mirrors what App.Run's event loop does with
// HandleInput's returned Cmds — run each one and feed its Msg back
// through Dispatch — since headless tests drive the App directly and
// have to do that step themselves (see docs/DESIGN.md §10).
func handleAndRun(app *tui.App, e input.Event) {
	for _, c := range app.HandleInput(e) {
		app.Dispatch(c())
	}
}

func testCommands() []Command {
	return []Command{
		{Label: "Open File", Data: 1},
		{Label: "Save File", Data: 2},
		{Label: "Close Window", Data: 3},
	}
}

type paletteHostModel struct {
	open      bool
	selected  Command
	cancelled bool
}

func (m *paletteHostModel) Init() tui.Cmd { return nil }
func (m *paletteHostModel) Update(msg tui.Msg) (tui.Model, tui.Cmd) {
	switch v := msg.(type) {
	case Command:
		m.selected = v
		m.open = false
	case string:
		if v == "cancel" {
			m.cancelled = true
			m.open = false
		}
	}
	return m, nil
}
func (m *paletteHostModel) View() tui.Node {
	return tui.Box(layout.Horizontal,
		tui.Child(layout.Fill(1), tui.Focusable("bg", tui.Text("background", cell.Style{}), func(e input.Event) tui.Msg { return "bg" })),
		tui.Child(layout.Length(0), CommandPalette(testCommands(), CommandPaletteOptions{
			Theme: style.DefaultDark(), Open: m.open, Placeholder: "type a command",
			Width: 30, Height: 8,
			OnSelect: func(cmd Command) tui.Msg { return cmd },
			OnCancel: func() tui.Msg { return "cancel" },
		})),
	)
}

func TestCommandPaletteNotPaintedWhenClosed(t *testing.T) {
	m := &paletteHostModel{open: false}
	app := tui.NewApp(m, 40, 10)
	if strings.Contains(app.Buffer().String(), "Open File") {
		t.Error("closed CommandPalette should not appear in the frame")
	}
}

func TestCommandPaletteShowsAllCommandsWithEmptyQuery(t *testing.T) {
	m := &paletteHostModel{open: true}
	app := tui.NewApp(m, 40, 10)
	got := app.Buffer().String()
	for _, want := range []string{"Open File", "Save File", "Close Window"} {
		if !strings.Contains(got, want) {
			t.Errorf("Buffer missing %q:\n%s", want, got)
		}
	}
}

func TestCommandPaletteFiltersAsYouType(t *testing.T) {
	m := &paletteHostModel{open: true}
	app := tui.NewApp(m, 40, 10)

	for _, r := range "save" {
		app.HandleInput(input.KeyEvent{Rune: r})
	}
	got := app.Buffer().String()
	if !strings.Contains(got, "Save File") {
		t.Errorf("Buffer = %q, want \"Save File\" to remain after filtering by \"save\"", got)
	}
	if strings.Contains(got, "Open File") || strings.Contains(got, "Close Window") {
		t.Errorf("Buffer = %q, want non-matching commands filtered out", got)
	}
}

func TestCommandPaletteEnterSelectsHighlightedCommand(t *testing.T) {
	m := &paletteHostModel{open: true}
	app := tui.NewApp(m, 40, 10)

	for _, r := range "close" {
		handleAndRun(app, input.KeyEvent{Rune: r})
	}
	handleAndRun(app, input.KeyEvent{Key: input.KeyEnter})

	if m.selected.Label != "Close Window" {
		t.Errorf("selected = %+v, want \"Close Window\"", m.selected)
	}
	if m.open {
		t.Error("expected the palette to close after selection (per this test model's Update)")
	}
}

func TestCommandPaletteEscCancels(t *testing.T) {
	m := &paletteHostModel{open: true}
	app := tui.NewApp(m, 40, 10)
	handleAndRun(app, input.KeyEvent{Key: input.KeyEsc})

	if !m.cancelled {
		t.Error("expected OnCancel to fire on Esc")
	}
}

func TestCommandPaletteUpDownMovesResultCursor(t *testing.T) {
	m := &paletteHostModel{open: true}
	app := tui.NewApp(m, 40, 10)

	handleAndRun(app, input.KeyEvent{Key: input.KeyDown})
	handleAndRun(app, input.KeyEvent{Key: input.KeyEnter})
	if m.selected.Label != "Save File" {
		t.Errorf("selected = %+v, want \"Save File\" (second entry, sorted by insertion order with an empty query)", m.selected)
	}
}

func TestCommandPaletteClaimsFocusExclusivelyWhileOpen(t *testing.T) {
	m := &paletteHostModel{open: true}
	app := tui.NewApp(m, 40, 10)

	cmds := app.HandleInput(input.KeyEvent{Key: input.KeyEsc})
	if len(cmds) != 1 || cmds[0]() != "cancel" {
		t.Fatalf("expected the palette (not the background) to receive Esc, got %v", cmds)
	}
}

func TestCommandPaletteResetsQueryOnReopen(t *testing.T) {
	m := &paletteHostModel{open: true}
	app := tui.NewApp(m, 40, 10)
	for _, r := range "save" {
		app.HandleInput(input.KeyEvent{Rune: r})
	}

	m.open = false
	app.Dispatch("noop")
	m.open = true
	app.Dispatch("noop")

	got := app.Buffer().String()
	if !strings.Contains(got, "Open File") {
		t.Errorf("Buffer = %q, want the query reset so all commands show again", got)
	}
	// The cursor block always covers column 0 (CommandPalette has no
	// separate focused/unfocused state — it's exclusively interactive
	// the whole time it's open), so the placeholder's first rune ('t')
	// is legitimately hidden underneath it; check from "ype" on.
	if !strings.Contains(got, "ype a command") {
		t.Errorf("Buffer = %q, want the placeholder shown (empty query)", got)
	}
}
