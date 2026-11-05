package app

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestStageShot draws the stage pane of Now Playing with a song playing,
// for its looks; skip it unless NM_SHOTS says where the picture goes.
func TestStageShot(t *testing.T) {
	dir := os.Getenv("NM_SHOTS")
	if dir == "" {
		t.Skip()
	}
	a, tt, _ := playingApp()
	a.nowPlaying, a.npPane = true, paneStage
	a.pl.Seek(2 * time.Second)
	tt.Frame()
	os.MkdirAll(dir, 0o755)
	f, err := os.Create(filepath.Join(dir, "stage.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}
