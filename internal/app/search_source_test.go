package app

import (
	"testing"

	"github.com/simqiqi/simqiqi-music/internal/sidecar"
)

// A route that names no catalogue searches 网易云: provNone is the zero
// value of providerID, so a search that never chose a source must not be
// sent to a bridge provider named "none".
func TestRouteSourceDefaultsToNetease(t *testing.T) {
	if got := (route{page: pageSearch, query: "周杰伦"}).source(); got != provNetease {
		t.Errorf("a route without a source searches %v, want 网易云音乐", got)
	}
	if got := (route{page: pageSearch, query: "周杰伦", src: provQQ}).source(); got != provQQ {
		t.Errorf("a route naming QQ音乐 searches %v, want it", got)
	}
}

// The catalogues a search offers are 网易云 plus whichever bridge
// providers are signed in, in the order the app lists them.
func TestSearchSourcesListsSignedInBridges(t *testing.T) {
	a := newTestApp()

	if got := a.searchSources(); len(got) != 1 || got[0] != provNetease {
		t.Fatalf("with nobody signed in, the sources are %v, want only 网易云", got)
	}

	a.bridgeOf(provQQ).acc = &sidecar.Account{ID: "1", Name: "QQ号"}
	a.bridgeOf(provKugou).acc = &sidecar.Account{ID: "2", Name: "酷狗号"}
	got := a.searchSources()
	want := []providerID{provNetease, provKugou, provQQ} // bridgeProviders 的顺序：汽水/酷狗/QQ
	if len(got) != len(want) {
		t.Fatalf("the sources are %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("the sources are %v, want %v", got, want)
		}
	}
}
