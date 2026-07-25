package render

import (
	"strconv"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/term"
)

// styleEqualSGR reports whether a and b would produce the same SGR
// (colors/attributes/underline) output — deliberately ignoring
// Hyperlink, which is encoded separately as OSC 8, not SGR. Renderer
// diffs the two independently so a hyperlink-only change (or a
// style-only change within an unbroken hyperlink) doesn't pay for the
// other's bytes.
func styleEqualSGR(a, b cell.Style) bool {
	return a.Fg == b.Fg && a.Bg == b.Bg && a.UnderlineColor == b.UnderlineColor &&
		a.Underline == b.Underline && a.Attr == b.Attr
}

// appendSGRDiff appends the minimal "ESC [ ... m" sequence that
// transitions from's SGR state to to's, downsampling colors to level.
// It appends nothing if there's no SGR-relevant difference.
func appendSGRDiff(buf []byte, from, to cell.Style, level term.ColorLevel) []byte {
	var codes [][]byte

	bf := cell.AttrBold | cell.AttrFaint
	if fromBF, toBF := from.Attr&bf, to.Attr&bf; fromBF != toBF {
		if fromBF != 0 {
			// 22 resets both 1 (bold) and 2 (faint) in real terminals;
			// only needed when clearing a prior state, not when going
			// from "neither set" straight to one of them.
			codes = append(codes, []byte("22"))
		}
		if toBF&cell.AttrBold != 0 {
			codes = append(codes, []byte("1"))
		}
		if toBF&cell.AttrFaint != 0 {
			codes = append(codes, []byte("2"))
		}
	}
	for _, ac := range independentAttrCodes {
		fromSet := from.Attr&ac.bit != 0
		toSet := to.Attr&ac.bit != 0
		if fromSet != toSet {
			if toSet {
				codes = append(codes, ac.set)
			} else {
				codes = append(codes, ac.reset)
			}
		}
	}

	if from.Underline != to.Underline {
		codes = append(codes, underlineCode(to.Underline))
	}
	if from.UnderlineColor != to.UnderlineColor {
		codes = append(codes, colorCode(to.UnderlineColor, level, 58)...)
	}
	if from.Fg != to.Fg {
		codes = append(codes, colorCode(to.Fg, level, 38)...)
	}
	if from.Bg != to.Bg {
		codes = append(codes, colorCode(to.Bg, level, 48)...)
	}

	if len(codes) == 0 {
		return buf
	}

	buf = append(buf, "\x1b["...)
	for i, c := range codes {
		if i > 0 {
			buf = append(buf, ';')
		}
		buf = append(buf, c...)
	}
	return append(buf, 'm')
}

type attrCode struct {
	bit        cell.Attr
	set, reset []byte
}

var independentAttrCodes = []attrCode{
	{cell.AttrItalic, []byte("3"), []byte("23")},
	{cell.AttrBlink, []byte("5"), []byte("25")},
	{cell.AttrReverse, []byte("7"), []byte("27")},
	{cell.AttrInvisible, []byte("8"), []byte("28")},
	{cell.AttrStrikethrough, []byte("9"), []byte("29")},
}

func underlineCode(u cell.UnderlineStyle) []byte {
	switch u {
	case cell.UnderlineNone:
		return []byte("24")
	case cell.UnderlineDouble:
		return []byte("4:2")
	case cell.UnderlineCurly:
		return []byte("4:3")
	case cell.UnderlineDotted:
		return []byte("4:4")
	case cell.UnderlineDashed:
		return []byte("4:5")
	default: // UnderlineSingle
		return []byte("4")
	}
}

// colorCode encodes c as one or more SGR parameter codes, downsampling
// per level. kind is 38 (foreground), 48 (background), or 58
// (underline color) — the "default" and basic-16 forms differ between
// foreground/background/underline, so kind selects among them.
func colorCode(c cell.Color, level term.ColorLevel, kind int) [][]byte {
	switch c.Kind {
	case cell.ColorKindDefault:
		return [][]byte{defaultColorCode(kind)}
	case cell.ColorKindANSI:
		return ansiColorCode(c.R, kind)
	case cell.ColorKindIndexed:
		n := c.R
		if level < term.Color256 {
			return ansiColorCode(downsampleIndexedTo16(n), kind)
		}
		return extendedIndexedCode(kind, n)
	case cell.ColorKindRGB:
		if level >= term.ColorTrueColor {
			return extendedRGBCode(kind, c.R, c.G, c.B)
		}
		idx := rgbToIndexed256(c.R, c.G, c.B)
		if level < term.Color256 {
			return ansiColorCode(downsampleIndexedTo16(idx), kind)
		}
		return extendedIndexedCode(kind, idx)
	}
	return nil
}

func defaultColorCode(kind int) []byte {
	switch kind {
	case 38:
		return []byte("39")
	case 48:
		return []byte("49")
	default: // 58: underline color
		return []byte("59")
	}
}

// ansiColorCode encodes n (0-15) as a basic (30-37/40-47) or bright
// (90-97/100-107) SGR code. Underline color (kind 58) has no basic-16
// form — CSI 58 always takes an extended (5/2) argument — so a
// downsampled underline color falls back to the indexed form instead.
func ansiColorCode(n uint8, kind int) [][]byte {
	if kind == 58 {
		return extendedIndexedCode(kind, n)
	}
	base := 30
	if kind == 48 {
		base = 40
	}
	if n < 8 {
		return [][]byte{appendInt(nil, base+int(n))}
	}
	brightBase := 90
	if kind == 48 {
		brightBase = 100
	}
	return [][]byte{appendInt(nil, brightBase+int(n)-8)}
}

// extendedIndexedCode and extendedRGBCode always use the colon form
// (e.g. "38:5:200", "38:2:255:0:0"), never the legacy all-semicolon
// form. This isn't just a style choice: when a sequence sets both
// foreground and background with the legacy form, "38;2;r;g;b;48;2;
// r;g;b" is genuinely ambiguous — nothing marks where the first color
// spec's fields end and the second's begin, since every field is a
// bare number. The colon form scopes each color spec to one field, so
// multiple can safely coexist in one SGR sequence. Since this encoder
// controls its own output, there's no reason to ever emit the unsafe
// form.
func extendedIndexedCode(kind int, n uint8) [][]byte {
	buf := appendInt(nil, kind)
	buf = append(buf, ':', '5', ':')
	buf = appendInt(buf, int(n))
	return [][]byte{buf}
}

func extendedRGBCode(kind int, r, g, b uint8) [][]byte {
	buf := appendInt(nil, kind)
	buf = append(buf, ':', '2', ':')
	buf = appendInt(buf, int(r))
	buf = append(buf, ':')
	buf = appendInt(buf, int(g))
	buf = append(buf, ':')
	buf = appendInt(buf, int(b))
	return [][]byte{buf}
}

func appendInt(buf []byte, n int) []byte {
	return strconv.AppendInt(buf, int64(n), 10)
}

// appendHyperlink appends an OSC 8 sequence opening (non-empty uri) or
// closing (empty uri) a hyperlink.
func appendHyperlink(buf []byte, uri string) []byte {
	buf = append(buf, "\x1b]8;;"...)
	buf = append(buf, uri...)
	return append(buf, '\x07')
}
