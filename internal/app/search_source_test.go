package app

import (
	"testing"

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

// The catalogues a search offers are 网易云 and every bridge provider, in
// the order the app lists them: a source nobody is signed in to is still
// offered, because the page asks for the sign-in itself.
func TestSearchSourcesOfferEveryCatalogue(t *testing.T) {
	a := newTestApp()
	got := a.searchSources()
	want := []providerID{provNetease, provQishui, provKugou, provQQ}
	if len(got) != len(want) {
		t.Fatalf("the sources are %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("the sources are %v, want %v", got, want)
		}
	}
}
