package vt

import "github.com/sandgorgon/tui/cell"

// sgr applies a Select Graphic Rendition sequence (CSI ... m) to the
// current style, used for all subsequent Print calls. No params (or a
// single 0) resets to the default style.
func (s *Screen) sgr(params CSIParams) {
	groups := params.Groups()
	if len(groups) == 0 {
		s.curStyle = cell.Style{}
		return
	}

	for i := 0; i < len(groups); i++ {
		g := groups[i]
		p := g[0]
		switch {
		case p == 0:
			s.curStyle = cell.Style{}
		case p == 1:
			s.curStyle.Attr |= cell.AttrBold
		case p == 2:
			s.curStyle.Attr |= cell.AttrFaint
		case p == 3:
			s.curStyle.Attr |= cell.AttrItalic
		case p == 4:
			if len(g) > 1 {
				s.curStyle.Underline = underlineStyleFromSub(g[1])
			} else {
				s.curStyle.Underline = cell.UnderlineSingle
			}
		case p == 5:
			s.curStyle.Attr |= cell.AttrBlink
		case p == 7:
			s.curStyle.Attr |= cell.AttrReverse
		case p == 8:
			s.curStyle.Attr |= cell.AttrInvisible
		case p == 9:
			s.curStyle.Attr |= cell.AttrStrikethrough
		case p == 21:
			s.curStyle.Underline = cell.UnderlineDouble // widely-implemented alternate meaning of SGR 21
		case p == 22:
			s.curStyle.Attr &^= cell.AttrBold | cell.AttrFaint
		case p == 23:
			s.curStyle.Attr &^= cell.AttrItalic
		case p == 24:
			s.curStyle.Underline = cell.UnderlineNone
		case p == 25:
			s.curStyle.Attr &^= cell.AttrBlink
		case p == 27:
			s.curStyle.Attr &^= cell.AttrReverse
		case p == 28:
			s.curStyle.Attr &^= cell.AttrInvisible
		case p == 29:
			s.curStyle.Attr &^= cell.AttrStrikethrough
		case p >= 30 && p <= 37:
			s.curStyle.Fg = cell.ANSIColor(uint8(p - 30))
		case p == 38:
			i += parseExtendedColor(groups, i, func(c cell.Color) { s.curStyle.Fg = c })
		case p == 39:
			s.curStyle.Fg = cell.DefaultColor()
		case p >= 40 && p <= 47:
			s.curStyle.Bg = cell.ANSIColor(uint8(p - 40))
		case p == 48:
			i += parseExtendedColor(groups, i, func(c cell.Color) { s.curStyle.Bg = c })
		case p == 49:
			s.curStyle.Bg = cell.DefaultColor()
		case p == 58:
			i += parseExtendedColor(groups, i, func(c cell.Color) { s.curStyle.UnderlineColor = c })
		case p == 59:
			s.curStyle.UnderlineColor = cell.DefaultColor()
		case p >= 90 && p <= 97:
			s.curStyle.Fg = cell.ANSIColor(uint8(p-90) + 8)
		case p >= 100 && p <= 107:
			s.curStyle.Bg = cell.ANSIColor(uint8(p-100) + 8)
		}
	}
}

func underlineStyleFromSub(n int) cell.UnderlineStyle {
	switch n {
	case 0:
		return cell.UnderlineNone
	case 2:
		return cell.UnderlineDouble
	case 3:
		return cell.UnderlineCurly
	case 4:
		return cell.UnderlineDotted
	case 5:
		return cell.UnderlineDashed
	default:
		return cell.UnderlineSingle
	}
}

// parseExtendedColor handles SGR 38/48/58 (set fg/bg/underline-color to
// an extended color), which appears in the wild in two different
// forms: the modern colon form — "38:2:r:g:b" or "38:5:n", all within
// ONE semicolon-separated group, sometimes with an empty colorspace-id
// sub-field before the RGB values ("38:2::r:g:b", per ITU T.416) — and
// the legacy semicolon form, "38;2;r;g;b" or "38;5;n", spread across
// SEPARATE groups. It calls set with the resulting color and returns
// how many additional groups (beyond the current one, at index i) the
// legacy form consumed, so the caller's loop index can skip them; the
// colon form always returns 0 since nothing beyond group i is touched.
func parseExtendedColor(groups [][]int, i int, set func(cell.Color)) int {
	g := groups[i]
	if len(g) > 1 {
		sub := g[1:] // [mode, ...]
		switch sub[0] {
		case 5:
			if len(sub) >= 2 {
				set(cell.IndexedColor(uint8(sub[1])))
			}
		case 2:
			// sub[1:] is either [r,g,b] or [colorspace,r,g,b].
			rgb := sub[1:]
			if len(rgb) >= 4 {
				rgb = rgb[len(rgb)-3:]
			}
			if len(rgb) >= 3 {
				set(cell.RGBColor(uint8(rgb[0]), uint8(rgb[1]), uint8(rgb[2])))
			}
		}
		return 0
	}

	// Legacy form: mode and its arguments are subsequent groups.
	if i+1 >= len(groups) {
		return 0
	}
	switch groups[i+1][0] {
	case 5:
		if i+2 >= len(groups) {
			return 1
		}
		set(cell.IndexedColor(uint8(groups[i+2][0])))
		return 2
	case 2:
		// Tolerate xterm's optional colorspace-id field here too:
		// "38;2;r;g;b" (4 groups after 38) or "38;2;CS;r;g;b" (5).
		if i+5 < len(groups) {
			set(cell.RGBColor(uint8(groups[i+3][0]), uint8(groups[i+4][0]), uint8(groups[i+5][0])))
			return 5
		}
		if i+4 < len(groups) {
			set(cell.RGBColor(uint8(groups[i+2][0]), uint8(groups[i+3][0]), uint8(groups[i+4][0])))
			return 4
		}
		return 1
	}
	return 1
}
