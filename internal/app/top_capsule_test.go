package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// The top capsule clears the caption buttons: Windows draws them at the
// top right of the hidden title bar, over where the capsule used to
// reach, and the avatar read as overlapping the close button.
func TestPageTopClearsCaptionButtons(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.profile = &netease.Profile{UserID: 1, Nickname: "听众"}
	tt := ui.NewTester(func(c *ui.Context) { a.view(c) }, 1180, 780)
	// Windows: three caption buttons, 46 DIP a piece.
	tt.SetTitleBar(ui.TitleBar{Height: 32, Right: 138})
	tt.Frame()
	box, ok := tt.Find("听众")
	if !ok {
		t.Fatal("no account button in the top bar")
	}
	if right := box.X + box.W; right > 1180-138 {
		t.Fatalf("account button reaches %.0f, past the caption buttons' edge %v", right, 1180-138)
	}
}
