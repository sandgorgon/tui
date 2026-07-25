package main

import (
	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/vt"
)

var dividerStyle = cell.Style{Fg: cell.ANSIColor(8)}

// compositeFrame blits every pane's vt.Screen buffer into host at the
// pane's rect and draws the divider between them. It returns the
// focused pane's cursor, translated into host coordinates, for the
// caller to pass to render.Renderer — the unfocused pane's cursor
// simply isn't shown, which doubles as this prototype's only visual
// focus indicator.
//
// Cells are copied with Buffer.Set directly rather than through a
// cell.Painter: a pane's cells are already fully resolved (correct
// Width, continuation cells and all) by vt.Screen itself, so this is a
// 1:1 blit, not "draw new text" — Painter's width-from-rune computation
// would be redundant here.
func compositeFrame(host *cell.Buffer, panes []*Pane, focused int) (cursorX, cursorY int, cursorVisible bool) {
	for i, p := range panes {
		var src *cell.Buffer
		var cx, cy int
		var cvis bool
		p.withScreen(func(s *vt.Screen) {
			src = s.Buffer()
			cx, cy, cvis = s.Cursor()
		})

		for y := range p.rect.H {
			for x := range p.rect.W {
				host.Set(p.rect.X+x, p.rect.Y+y, src.At(x, y))
			}
		}

		if i == focused {
			cursorX, cursorY, cursorVisible = p.rect.X+cx, p.rect.Y+cy, cvis
		}
	}

	drawDivider(host, panes)
	return
}

func drawDivider(host *cell.Buffer, panes []*Pane) {
	if len(panes) < 2 {
		return
	}
	x := panes[0].rect.W
	for y := range host.Height {
		host.Set(x, y, cell.Cell{Rune: '│', Width: 1, Style: dividerStyle})
	}
}
