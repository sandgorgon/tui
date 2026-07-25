package render

import (
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/term"
)

func TestAppendSGRDiffNoChange(t *testing.T) {
	got := appendSGRDiff(nil, cell.Style{}, cell.Style{}, term.ColorTrueColor)
	if len(got) != 0 {
		t.Errorf("appendSGRDiff(zero, zero) = %q, want empty", got)
	}
}

func TestAppendSGRDiff(t *testing.T) {
	tests := []struct {
		name     string
		from, to cell.Style
		level    term.ColorLevel
		want     string
	}{
		{
			name: "set bold",
			from: cell.Style{}, to: cell.Style{Attr: cell.AttrBold},
			level: term.ColorTrueColor, want: "\x1b[1m",
		},
		{
			name: "clear bold",
			from: cell.Style{Attr: cell.AttrBold}, to: cell.Style{},
			level: term.ColorTrueColor, want: "\x1b[22m",
		},
		{
			name: "ansi fg color ignores level",
			from: cell.Style{}, to: cell.Style{Fg: cell.ANSIColor(1)},
			level: term.ColorTrueColor, want: "\x1b[31m",
		},
		{
			name: "truecolor fg at ColorTrueColor",
			from: cell.Style{}, to: cell.Style{Fg: cell.RGBColor(255, 0, 0)},
			level: term.ColorTrueColor, want: "\x1b[38:2:255:0:0m",
		},
		{
			name: "truecolor fg downsampled to 256",
			from: cell.Style{}, to: cell.Style{Fg: cell.RGBColor(255, 0, 0)},
			level: term.Color256, want: "\x1b[38:5:196m",
		},
		{
			name: "truecolor fg downsampled to 16 (bright red)",
			from: cell.Style{}, to: cell.Style{Fg: cell.RGBColor(255, 0, 0)},
			level: term.Color16, want: "\x1b[91m",
		},
		{
			name: "curly underline",
			from: cell.Style{}, to: cell.Style{Underline: cell.UnderlineCurly},
			level: term.ColorTrueColor, want: "\x1b[4:3m",
		},
		{
			name: "clear underline",
			from: cell.Style{Underline: cell.UnderlineSingle}, to: cell.Style{},
			level: term.ColorTrueColor, want: "\x1b[24m",
		},
		{
			name: "underline color",
			from: cell.Style{}, to: cell.Style{UnderlineColor: cell.RGBColor(1, 2, 3)},
			level: term.ColorTrueColor, want: "\x1b[58:2:1:2:3m",
		},
		{
			name: "multiple changes combine into one sequence",
			from: cell.Style{}, to: cell.Style{Attr: cell.AttrBold, Fg: cell.ANSIColor(2)},
			level: term.ColorTrueColor, want: "\x1b[1;32m",
		},
		{
			name: "bright bg color",
			from: cell.Style{}, to: cell.Style{Bg: cell.ANSIColor(9)},
			level: term.ColorTrueColor, want: "\x1b[101m",
		},
		{
			name: "default fg resets to 39",
			from: cell.Style{Fg: cell.ANSIColor(1)}, to: cell.Style{},
			level: term.ColorTrueColor, want: "\x1b[39m",
		},
		{
			name: "indexed color",
			from: cell.Style{}, to: cell.Style{Fg: cell.IndexedColor(200)},
			level: term.Color256, want: "\x1b[38:5:200m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(appendSGRDiff(nil, tt.from, tt.to, tt.level))
			if got != tt.want {
				t.Errorf("appendSGRDiff = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStyleEqualSGRIgnoresHyperlink(t *testing.T) {
	a := cell.Style{Fg: cell.ANSIColor(1), Hyperlink: "https://a"}
	b := cell.Style{Fg: cell.ANSIColor(1), Hyperlink: "https://b"}
	if !styleEqualSGR(a, b) {
		t.Error("styleEqualSGR should ignore Hyperlink and report these equal")
	}
	c := cell.Style{Fg: cell.ANSIColor(2), Hyperlink: "https://a"}
	if styleEqualSGR(a, c) {
		t.Error("styleEqualSGR should report a genuine Fg difference as unequal")
	}
}

func TestAppendHyperlink(t *testing.T) {
	got := string(appendHyperlink(nil, "https://example.com"))
	want := "\x1b]8;;https://example.com\x07"
	if got != want {
		t.Errorf("appendHyperlink = %q, want %q", got, want)
	}
	closed := string(appendHyperlink(nil, ""))
	if closed != "\x1b]8;;\x07" {
		t.Errorf("appendHyperlink(\"\") = %q, want closing sequence", closed)
	}
}
