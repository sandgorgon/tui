package style

import (
	"testing"

	"github.com/sandgorgon/tui/cell"
)

func TestDefaultRespectsAppearance(t *testing.T) {
	if got := Default(Dark).Appearance; got != Dark {
		t.Errorf("Default(Dark).Appearance = %v, want Dark", got)
	}
	if got := Default(Light).Appearance; got != Light {
		t.Errorf("Default(Light).Appearance = %v, want Light", got)
	}
}

func TestDefaultThemesLeaveForegroundBackgroundAtTerminalDefault(t *testing.T) {
	for _, th := range []Theme{DefaultDark(), DefaultLight()} {
		if th.Foreground != cell.DefaultColor() {
			t.Errorf("%v theme Foreground = %+v, want the zero/default color", th.Appearance, th.Foreground)
		}
		if th.Background != cell.DefaultColor() {
			t.Errorf("%v theme Background = %+v, want the zero/default color", th.Appearance, th.Background)
		}
	}
}

func TestDefaultThemesSetEverySemanticRole(t *testing.T) {
	zero := cell.Color{}
	for _, th := range []Theme{DefaultDark(), DefaultLight()} {
		roles := map[string]cell.Color{
			"Primary": th.Primary, "Secondary": th.Secondary, "Accent": th.Accent,
			"Muted": th.Muted, "Border": th.Border, "Focus": th.Focus,
			"Success": th.Success, "Warning": th.Warning, "Error": th.Error, "Info": th.Info,
		}
		for name, c := range roles {
			if c == zero {
				t.Errorf("%v theme's %s role is the zero Color (unset)", th.Appearance, name)
			}
		}
	}
}

func TestThemeStyleHelpersUseExpectedRoles(t *testing.T) {
	th := DefaultDark()

	if got := th.Text(); got.Fg != th.Foreground || got.Bg != th.Background {
		t.Errorf("Text() = %+v", got)
	}
	if got := th.MutedText(); got.Fg != th.Muted {
		t.Errorf("MutedText().Fg = %+v, want theme.Muted", got.Fg)
	}
	if got := th.BorderStyle(); got.Fg != th.Border {
		t.Errorf("BorderStyle().Fg = %+v, want theme.Border", got.Fg)
	}
	focus := th.FocusStyle()
	if focus.Fg != th.Focus {
		t.Errorf("FocusStyle().Fg = %+v, want theme.Focus", focus.Fg)
	}
	if focus.Attr&cell.AttrBold == 0 {
		t.Error("FocusStyle() should be bold to stand out from BorderStyle()")
	}
}
