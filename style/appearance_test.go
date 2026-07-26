package style

import "testing"

func TestDetectAppearance(t *testing.T) {
	tests := []struct {
		name      string
		colorfgbg string
		want      Appearance
	}{
		{"unset", "", Dark},
		{"no semicolon", "garbage", Dark},
		{"non-numeric bg", "0;x", Dark},
		{"dark bg (black, 0)", "15;0", Dark},
		{"dark bg (blue, 4)", "15;4", Dark},
		{"light bg (white, 7)", "0;7", Light},
		{"light bg (bright white, 15)", "0;15", Light},
		{"multi-part fg (kitty-style)", "15;default;0", Dark},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv := func(key string) string {
				if key == "COLORFGBG" {
					return tt.colorfgbg
				}
				return ""
			}
			if got := DetectAppearance(getenv); got != tt.want {
				t.Errorf("DetectAppearance(COLORFGBG=%q) = %v, want %v", tt.colorfgbg, got, tt.want)
			}
		})
	}
}

func TestAppearanceString(t *testing.T) {
	if Dark.String() != "Dark" {
		t.Errorf("Dark.String() = %q", Dark.String())
	}
	if Light.String() != "Light" {
		t.Errorf("Light.String() = %q", Light.String())
	}
}
