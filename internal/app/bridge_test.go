package app

import (
	"testing"
	"time"

	"github.com/simqiqi/simqiqi-music/internal/sidecar"
)

// The sidebar asks for an account's rows every frame, so they are built
// when its playlists come in and kept: asking again, with the same lists,
// hands back what was built, without taking the registry's lock per row.
func TestBridgePlaylistVMsKeptWhileListsHold(t *testing.T) {
	a := &app{bridge: newBridgeStore()}
	lists := []sidecar.BridgePlaylist{
		{ID: "1", Name: "我喜欢的音乐", Cover: "https://p3.douyinpic.com/a.jpg", Count: 20},
		{ID: "2", Name: "通勤", Cover: "https://p3.douyinpic.com/b.jpg", Count: 8},
	}
	first := a.bridgePlaylistVMs(provQishui, lists)
	if len(first) != 2 || first[0].Name != "我喜欢的音乐" || first[1].TrackCount != 8 {
		t.Fatalf("rows = %v, want the two playlists", first)
	}
	next := a.bridge.next
	// A new slice with the same content, as each frame's copy of the
	// account is not the one before it.
	same := append([]sidecar.BridgePlaylist(nil), lists...)
	again := a.bridgePlaylistVMs(provQishui, same)
	if len(again) != 2 || again[0].ID != first[0].ID || again[1].ID != first[1].ID {
		t.Errorf("rows changed: %v, then %v", first, again)
	}
	if a.bridge.next != next {
		t.Errorf("assigned IDs again: next went from %d to %d", next, a.bridge.next)
	}

	// A playlist renamed, as it is on the provider, rebuilds the rows.
	renamed := append([]sidecar.BridgePlaylist(nil), lists...)
	renamed[1].Name = "通勤路上"
	got := a.bridgePlaylistVMs(provQishui, renamed)
	if got[1].Name != "通勤路上" || got[1].ID != first[1].ID {
		t.Errorf("renamed rows = %v, want 通勤路上 with its ID", got[1])
	}
}

// TestBridgeSongLookupDoesNotDeadlock guards the bridge registry's lock:
// looking a song up reads two maps, and the provider must come from the
// same lock — calling providerOf while holding it would lock twice, and
// sync.Mutex does not nest, which wedged playback at «正在缓冲» once.
func TestBridgeSongLookupDoesNotDeadlock(t *testing.T) {
	a := &app{bridge: newBridgeStore()}
	bt := sidecar.BridgeTrack{ID: "7442146553232820240", Name: "那个女孩", Artist: "宝石Gem"}
	id := a.bridge.trackID(provQishui, bt)

	done := make(chan bool, 1)
	go func() {
		got, prov, ok := a.bridgeSong(id)
		if !ok || got.Name != bt.Name || prov != provQishui {
			t.Errorf("bridgeSong = %v, %v, %v; want the track, qishui, true", got, prov, ok)
		}
		// 两次连续查找，第二次若有人把 providerOf 放回锁内就会卡死。
		if _, _, ok := a.bridgeSong(id); !ok {
			t.Error("second lookup lost the song")
		}
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("bridgeSong deadlocked: the registry lock was taken twice")
	}
}
