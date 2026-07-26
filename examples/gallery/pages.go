package main

import (
	"fmt"

	"github.com/sandgorgon/tui/layout"
	"github.com/sandgorgon/tui/tui"
	"github.com/sandgorgon/tui/widget"
)

func (m *model) View() tui.Node {
	var page tui.Node
	switch m.page {
	case pageText:
		page = m.viewText()
	case pageData:
		page = m.viewData()
	case pageForms:
		page = m.viewForms()
	case pageTerminal:
		page = m.viewTerminal()
	}

	help := fmt.Sprintf(
		"Tab: focus  1-4/←→/click: page  F1: palette  F2: modal  F3: copy status  Ctrl+Q: quit  |  %s",
		m.status,
	)

	root := tui.Box(layout.Vertical,
		tui.Child(layout.Length(1), widget.Tabs(pageLabels, m.page, m.theme, tabsEvent)),
		tui.Child(layout.Fill(1), page),
		tui.Child(layout.Length(1), widget.StatusBar(nil, nil,
			[]widget.Segment{{Text: help}}, m.theme.MutedText())),
	).Margin(1)

	return tui.Box(layout.Horizontal,
		tui.Child(layout.Fill(1), root),
		tui.Child(layout.Length(0), m.viewCommandPalette()),
		tui.Child(layout.Length(0), m.viewModal()),
	)
}

func (m *model) viewText() tui.Node {
	return tui.Box(layout.Vertical,
		tui.Child(layout.Length(4), widget.Paragraph(
			"This page shows the M10 text/feedback widgets: Paragraph "+
				"(this text, word-wrapped), ProgressBar, and Spinner. Both "+
				"animate off a periodic tick this app schedules itself — see "+
				"widget.Spinner's doc comment on why a change-driven-redraw "+
				"library needs that.",
			m.theme.Text(),
		)),
		tui.Child(layout.Length(1), widget.ProgressBar(m.progress, widget.ProgressBarOptions{Theme: m.theme})),
		tui.Child(layout.Length(1), tui.Box(layout.Horizontal,
			tui.Child(layout.Length(3), widget.Spinner(widget.SpinnerOptions{Style: m.theme.Text()})),
			tui.Child(layout.Fill(1), tui.Text(" working...", m.theme.MutedText())),
		)),
	).Gap(1)
}

func (m *model) viewData() tui.Node {
	return tui.Box(layout.Horizontal,
		tui.Child(layout.Fill(1), widget.List(m.listItems, m.listCursor,
			widget.ListOptions{Theme: m.theme, Selected: m.listSelected}, listEvent)),
		tui.Child(layout.Fill(1), tui.Box(layout.Vertical,
			tui.Child(layout.Fill(1), widget.Tree(m.treeRows, m.treeCursor, m.theme, treeEvent)),
			tui.Child(layout.Length(7), widget.Table(m.tableCols, m.tableRows, m.tableCursor,
				widget.TableOptions{Theme: m.theme, SortColumn: m.tableSortCol, SortDescending: m.tableSortDesc},
				tableEvent)),
		)),
	).Gap(1)
}

func (m *model) viewForms() tui.Node {
	return tui.Box(layout.Vertical,
		tui.Child(layout.Length(3), widget.TextInput(widget.TextInputOptions{
			Theme: m.theme, Placeholder: "type here, then Enter",
			OnSubmit: func(v string) tui.Msg { m.lastSubmitted = v; return nil },
		})),
		tui.Child(layout.Length(4), widget.TextArea(widget.TextAreaOptions{
			Theme: m.theme, Placeholder: "multi-line notes",
		})),
		tui.Child(layout.Fill(1), tui.Box(layout.Horizontal,
			tui.Child(layout.Fill(1), widget.RadioGroup(
				m.radioOptions, m.radioSelected, m.theme, radioEvent)),
			tui.Child(layout.Fill(1), widget.CheckboxGroup(
				m.checkboxOptions, m.checkboxChecked, m.checkboxCursor, m.theme, checkboxEvent)),
			tui.Child(layout.Fill(1), widget.Select(
				m.selectOptions, m.selectSelected, m.selectCursor,
				widget.SelectOptions{Theme: m.theme, Open: m.selectOpen}, selectEvent)),
		)),
	).Gap(1)
}

func (m *model) viewTerminal() tui.Node {
	return tui.Box(layout.Vertical,
		tui.Child(layout.Length(1), tui.Text(
			"A live widget.Terminal pane (M11) running your $SHELL — pty+vt wired as a normal widget, not the M6 hand-rolled prototype examples/multiplexer still uses.",
			m.theme.MutedText())),
		tui.Child(layout.Fill(1), widget.Terminal(widget.TerminalOptions{Command: m.shellCmd()})),
	)
}

func (m *model) viewCommandPalette() tui.Node {
	return widget.CommandPalette(
		[]widget.Command{
			{Label: "Go to Text & Feedback page", Data: pageText},
			{Label: "Go to Lists & Data page", Data: pageData},
			{Label: "Go to Forms page", Data: pageForms},
			{Label: "Go to Terminal page", Data: pageTerminal},
			{Label: "Reset demo state"},
		},
		widget.CommandPaletteOptions{
			Theme: m.theme, Open: m.paletteOpen, Placeholder: "type a command",
			Width: 40, Height: 10,
			OnSelect: func(cmd widget.Command) tui.Msg { return cmd },
			OnCancel: func() tui.Msg { return "palette-cancel" },
		},
	)
}

func (m *model) viewModal() tui.Node {
	body := tui.Box(layout.Vertical,
		tui.Child(layout.Length(2), tui.Text("Reset all demo state?", m.theme.Text())),
		tui.Child(layout.Length(3), tui.Focusable("modal-ok",
			tui.Text("[ OK ]", m.theme.Text()), modalButtonEvent(true))),
		tui.Child(layout.Length(3), tui.Focusable("modal-cancel",
			tui.Text("[ Cancel ]", m.theme.Text()), modalButtonEvent(false))),
	).Gap(1)

	return widget.Modal(body, widget.ModalOptions{
		Theme: m.theme, Open: m.modalOpen, Title: "Confirm", Width: 30, Height: 12,
	})
}
