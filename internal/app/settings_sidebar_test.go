package app

import (
	"os"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// The settings' sidebar group lists the directory's nine entries as
// chips: a press hides one from the column, and a second shows it again.
func TestSettingsSidebarChips(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	// No app runs to apply an appearance to.
	old := setThemeSource
	setThemeSource = func(mygo.ThemeSource) {}
	defer func() { setThemeSource = old }()

	tt := ui.NewTester(a.settingsTestView, 560, 2000)
	tt.Frame()
	if dir := os.Getenv("NM_SHOTS"); dir != "" {
		saveFrame(t, dir, "settings-sidebar.png", tt.Image())
	}
	for _, label := range []string{
		"为你推荐", "每日推荐", "听歌周报", "私人 FM", "天气电台",
		"排行榜", "播客", "下载管理", "新碟速递",
	} {
		if !tt.HasText(label) {
			t.Fatalf("%q missing from the settings", label)
		}
	}
	// The vinyl switch is gone from the settings: the record button on
	// the cover still turns the record on and off.
	if tt.HasText("黑胶唱片") {
		t.Fatal("the vinyl switch is still in the settings")
	}
	if err := tt.Click("私人 FM"); err != nil {
		t.Fatal(err)
	}
	if a.settings.SidebarFM {
		t.Fatal("private FM still shown after a press")
	}
	if err := tt.Click("私人 FM"); err != nil {
		t.Fatal(err)
	}
	if !a.settings.SidebarFM {
		t.Fatal("private FM still hidden after a second press")
	}
}

// Hidden at the directory, an entry is gone from the column.
func TestSidebarEntriesFollowTheChips(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.settings.SidebarWeekly = false
	a.settings.SidebarDownloads = false
	tt := ui.NewTester(a.view, 1200, 720)
	tt.Frame()
	if tt.HasText("听歌周报") {
		t.Fatal("听歌周报 drawn though hidden")
	}
	if tt.HasText("下载管理") {
		t.Fatal("下载管理 drawn though hidden")
	}
	if !tt.HasText("为你推荐") {
		t.Fatal("为你推荐 missing though shown")
	}
}

// The "动态收缩" switch in the sidebar group turns the folding directory
// on and off, and the choice is kept on the settings.
func TestSettingsDynamicCollapseSwitch(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	old := setThemeSource
	setThemeSource = func(mygo.ThemeSource) {}
	defer func() { setThemeSource = old }()

	tt := ui.NewTester(a.settingsTestView, 560, 2200)
	tt.Frame()
	title, ok := tt.Find("动态收缩")
	if !ok {
		t.Fatal("the 动态收缩 switch is missing from the settings")
	}
	// The switch sits in the row with the text that names it, on its
	// right: at its middle height.
	y := title.Y + title.H/2

	flip := func() bool {
		before := a.settings.sidebarAutoCollapse()
		for x := float32(460); x < 545; x += 2 {
			tt.ClickAt(x, y)
			tt.Frame()
			if a.settings.sidebarAutoCollapse() != before {
				return true
			}
		}
		return false
	}
	if !flip() {
		t.Fatal("the switch took no press")
	}
	if a.settings.sidebarAutoCollapse() || a.settings.SidebarAutoCollapse == nil {
		t.Fatal("a press should have turned the folding off")
	}
	if !flip() {
		t.Fatal("the second press did nothing")
	}
	if !a.settings.sidebarAutoCollapse() {
		t.Fatal("a second press should have turned it back on")
	}
}
