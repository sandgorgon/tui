package render

import "testing"

func TestRGBToIndexed256(t *testing.T) {
	tests := []struct {
		name    string
		r, g, b uint8
		want    uint8
	}{
		{"pure black favors cube corner", 0, 0, 0, 16},
		{"pure white favors cube corner", 255, 255, 255, 231},
		{"exact grayscale step wins over cube", 128, 128, 128, 244},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rgbToIndexed256(tt.r, tt.g, tt.b); got != tt.want {
				t.Errorf("rgbToIndexed256(%d,%d,%d) = %d, want %d", tt.r, tt.g, tt.b, got, tt.want)
			}
		})
	}
}

func TestRGBToIndexed16(t *testing.T) {
	tests := []struct {
		name    string
		r, g, b uint8
		want    uint8
	}{
		{"exact match: black", 0, 0, 0, 0},
		{"exact match: bright red", 255, 0, 0, 9},
		{"exact match: bright white", 255, 255, 255, 15},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rgbToIndexed16(tt.r, tt.g, tt.b); got != tt.want {
				t.Errorf("rgbToIndexed16(%d,%d,%d) = %d, want %d", tt.r, tt.g, tt.b, got, tt.want)
			}
		})
	}
}

func TestDownsampleIndexedTo16(t *testing.T) {
	if got := downsampleIndexedTo16(5); got != 5 {
		t.Errorf("downsampleIndexedTo16(5) = %d, want 5 (already in range, unchanged)", got)
	}
	// Palette index 16 is the cube's black corner (RGB 0,0,0), which
	// should downsample to basic color 0 (black).
	if got := downsampleIndexedTo16(16); got != 0 {
		t.Errorf("downsampleIndexedTo16(16) = %d, want 0", got)
	}
}

func TestIndexed256ToRGB(t *testing.T) {
	tests := []struct {
		n                   uint8
		wantR, wantG, wantB uint8
	}{
		{16, 0, 0, 0},        // cube black corner
		{231, 255, 255, 255}, // cube white corner
		{232, 8, 8, 8},       // start of grayscale ramp
	}
	for _, tt := range tests {
		r, g, b := indexed256ToRGB(tt.n)
		if r != tt.wantR || g != tt.wantG || b != tt.wantB {
			t.Errorf("indexed256ToRGB(%d) = (%d,%d,%d), want (%d,%d,%d)", tt.n, r, g, b, tt.wantR, tt.wantG, tt.wantB)
		}
	}
}
