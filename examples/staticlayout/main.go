// Command staticlayout is the M7 milestone demo: it builds a nested
// layout — a header/footer split around a body, with the body further
// split into a sidebar, a main pane, and a detail panel — using package
// layout's Constraint/Split solver, then paints one static frame
// showing the resulting Rects. There's no interaction and no raw mode:
// it prints a single frame and exits, proving Constraint, Split, Gap,
// Margin, and nesting end to end. A live App loop redrawing on input
// and resize is M8's job, not this one.
package main

import (
	"fmt"
	"os"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/layout"
	"github.com/sandgorgon/tui/render"
	"github.com/sandgorgon/tui/term"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "staticlayout:", err)
		os.Exit(1)
	}
}

func run() error {
	width, height := 80, 24
	if term.IsTerminal(os.Stdout) {
		if sz, err := term.GetSize(os.Stdout); err == nil {
			width, height = sz.Cols, sz.Rows
		}
	}

	root := layout.Rect{X: 0, Y: 0, W: width, H: height}
	rows := layout.New(layout.Vertical,
		layout.Length(3), // header
		layout.Fill(1),   // body
		layout.Length(1), // footer
	).Split(root)
	header, body, footer := rows[0], rows[1], rows[2]

	// sidebar: fixed width. main: gets 2x the leftover share of a plain
	// Fill. detail: shares leftover too, but never shrinks below 15
	// cells even when the terminal is narrow.
	cols := layout.New(layout.Horizontal,
		layout.Length(20),
		layout.Fill(2),
		layout.Min(15),
	).Gap(1).Split(body)
	sidebar, mainPane, detail := cols[0], cols[1], cols[2]

	panes := []struct {
		rect  layout.Rect
		title string
		desc  string
		color cell.Color
	}{
		{header, "header", "Length(3)", cell.ANSIColor(4)},
		{sidebar, "sidebar", "Length(20)", cell.ANSIColor(2)},
		{mainPane, "main", "Fill(2)", cell.ANSIColor(3)},
		{detail, "detail", "Min(15)", cell.ANSIColor(5)},
		{footer, "footer", "Length(1)", cell.ANSIColor(6)},
	}

	buf := cell.NewBuffer(width, height)
	root2 := cell.NewPainter(buf)
	for _, p := range panes {
		drawPane(root2, p.rect, p.title, p.desc, cell.Style{Fg: p.color})
	}

	caps := term.DetectEnv(os.Getenv)
	renderer := render.NewRenderer(render.Options{ColorLevel: caps.ColorLevel})

	os.Stdout.WriteString("\x1b[2J\x1b[H")
	// Park the cursor at the top-left rather than the bottom-right:
	// the shell's own post-exit prompt will print a newline, and doing
	// that from the terminal's last row would scroll the frame we just
	// drew before the user even sees it.
	if err := renderer.Render(os.Stdout, buf, 0, 0, true); err != nil {
		return fmt.Errorf("Render: %w", err)
	}
	return nil
}

// drawPane paints a labeled, single-line border around r using style,
// scoped to r via Painter.Clip so the drawing code works in r's own
// local coordinates regardless of where r sits in the buffer.
func drawPane(root *cell.Painter, r layout.Rect, title, desc string, style cell.Style) {
	p := root.Clip(cell.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H})
	w, h := p.Size()
	if w <= 0 || h <= 0 {
		return
	}

	if w >= 2 && h >= 2 {
		p.SetCell(0, 0, '┌', style)
		p.SetCell(w-1, 0, '┐', style)
		p.SetCell(0, h-1, '└', style)
		p.SetCell(w-1, h-1, '┘', style)
	}
	for x := 1; x < w-1; x++ {
		p.SetCell(x, 0, '─', style)
		p.SetCell(x, h-1, '─', style)
	}
	for y := 1; y < h-1; y++ {
		p.SetCell(0, y, '│', style)
		p.SetCell(w-1, y, '│', style)
	}

	switch {
	case h >= 3 && w >= 4:
		p.Text(2, 1, title, style)
		p.Text(2, 2, desc, style)
	case h >= 1 && w >= 3:
		p.Text(1, h/2, title, style)
	}
}
