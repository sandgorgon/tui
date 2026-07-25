//go:generate go run gen.go

package wcwidth

// Range is an inclusive code point range.
type Range struct {
	Lo, Hi rune
}

func inRanges(r rune, ranges []Range) bool {
	lo, hi := 0, len(ranges)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		switch {
		case r < ranges[mid].Lo:
			hi = mid - 1
		case r > ranges[mid].Hi:
			lo = mid + 1
		default:
			return true
		}
	}
	return false
}

// RuneWidth returns r's display width in terminal cells:
//
//   - -1 for non-printable control characters (C0, DEL, C1). These
//     should never reach this layer as literal cell content — a VT
//     parser interprets them as control actions, not printable output —
//     so this is a defensive classification, not an expected input.
//   - 0 for zero-width combining marks and format characters.
//   - 2 for wide East Asian and default-emoji-presentation characters.
//   - 1 for everything else, including "ambiguous width" East Asian
//     characters, which resolve to narrow by default — the same
//     convention most terminals use unless configured otherwise.
func RuneWidth(r rune) int {
	switch {
	case r == 0, r < 0x20, r >= 0x7F && r <= 0x9F:
		return -1
	case inRanges(r, zeroWidth):
		return 0
	case inRanges(r, wide):
		return 2
	default:
		return 1
	}
}
