package wcwidth

import "testing"

func TestRuneWidth(t *testing.T) {
	tests := []struct {
		name string
		r    rune
		want int
	}{
		{"NUL", 0x00, -1},
		{"tab", '\t', -1},
		{"escape", 0x1b, -1},
		{"DEL", 0x7f, -1},
		{"C1 control", 0x90, -1},
		{"space", ' ', 1},
		{"ascii letter", 'a', 1},
		{"ascii digit", '5', 1},
		{"latin accented", 'é', 1},
		{"combining acute accent", 0x0301, 0},
		{"combining enclosing circle (Me)", 0x20DD, 0},
		{"zero width space", 0x200B, 0},
		{"zero width joiner", 0x200D, 0},
		{"BOM / ZWNBSP", 0xFEFF, 0},
		{"variation selector 16", 0xFE0F, 0},
		{"hangul syllable (wide)", 0xAC00, 2},
		{"cjk unified ideograph", 0x4E2D, 2}, // 中
		{"hiragana", 0x3042, 2},              // あ
		{"katakana", 0x30A2, 2},              // ア
		{"fullwidth latin A", 0xFF21, 2},
		{"box drawing light horizontal", 0x2500, 1},
		{"emoji grinning face", 0x1F600, 2},
		{"emoji watch (E0.6 legacy)", 0x231A, 2},
		{"cjk punctuation ideographic comma", 0x3001, 2},
		{"private use area", 0xE000, 1},
		{"basic multilingual plane end", 0xFFFD, 1},
		{"supplementary plane cjk ext", 0x20000, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RuneWidth(tt.r); got != tt.want {
				t.Errorf("RuneWidth(%U) = %d, want %d", tt.r, got, tt.want)
			}
		})
	}
}

func TestTablesAreSortedAndDisjoint(t *testing.T) {
	checkSorted := func(name string, rs []Range) {
		t.Helper()
		for i := 1; i < len(rs); i++ {
			if rs[i].Lo <= rs[i-1].Hi {
				t.Errorf("%s: range %d (%#x-%#x) overlaps or is out of order with range %d (%#x-%#x)",
					name, i, rs[i].Lo, rs[i].Hi, i-1, rs[i-1].Lo, rs[i-1].Hi)
			}
			if rs[i].Lo > rs[i].Hi {
				t.Errorf("%s: range %d has Lo > Hi: %#x-%#x", name, i, rs[i].Lo, rs[i].Hi)
			}
		}
	}
	checkSorted("zeroWidth", zeroWidth)
	checkSorted("wide", wide)

	for _, z := range zeroWidth {
		if inRanges(z.Lo, wide) || inRanges(z.Hi, wide) {
			t.Errorf("zeroWidth range %#x-%#x overlaps the wide table", z.Lo, z.Hi)
		}
	}
}
