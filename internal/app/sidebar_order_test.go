package app

import (
	"reflect"
	"testing"
)

// TestSidebarOrderDefaults: with nothing saved the nine pages stand in
// their original order, and an unknown id in a saved order is ignored.
func TestSidebarOrderDefaults(t *testing.T) {
	s := defaultSettings()
	if got, want := s.sidebarOrder(), sidebarNavIDs; !reflect.DeepEqual(got, want) {
		t.Errorf("default order %v, want %v", got, want)
	}
	s.SidebarOrder = []string{"unknown", "downloads", "unknown"}
	got := s.sidebarOrder()
	if got[0] != "downloads" {
		t.Errorf("saved order not honoured: %v", got)
	}
	if len(got) != len(sidebarNavIDs) || reflect.DeepEqual(got, s.SidebarOrder) {
		t.Errorf("saved order not completed with the defaults: %v", got)
	}
}

// TestSidebarOrderCompletesNewPages: an order saved before a page existed
// must still list that page, or an update would quietly hide it.
func TestSidebarOrderCompletesNewPages(t *testing.T) {
	s := defaultSettings()
	s.SidebarOrder = []string{"home", "daily"}
	got := s.sidebarOrder()
	seen := map[string]bool{}
	for _, id := range got {
		seen[id] = true
	}
	for _, id := range sidebarNavIDs {
		if !seen[id] {
			t.Errorf("page %q vanished from a partial order: %v", id, got)
		}
	}
	if !reflect.DeepEqual(got[:2], []string{"home", "daily"}) {
		t.Errorf("the saved prefix moved: %v", got)
	}
}

// TestMoveSidebarEntry moves one page past another and can be undone.
func TestMoveSidebarEntry(t *testing.T) {
	s := defaultSettings()
	before := append([]string(nil), s.sidebarOrder()...)
	s.moveSidebarEntry(0, 2)
	after := s.sidebarOrder()
	if after[2] != "home" || after[0] != "daily" || after[1] != "weekly" {
		t.Errorf("moving the first page down two gave %v", after)
	}
	s.moveSidebarEntry(2, 0)
	if got := s.sidebarOrder(); !reflect.DeepEqual(got, before) {
		t.Errorf("moving it back gave %v, want %v", got, before)
	}
	// Out-of-range and no-op moves leave the order alone.
	for _, pair := range [][2]int{{-1, 0}, {0, 0}, {0, 99}} {
		s.moveSidebarEntry(pair[0], pair[1])
		if got := s.sidebarOrder(); !reflect.DeepEqual(got, before) {
			t.Errorf("move %v changed the order to %v", pair, got)
		}
	}
}

// TestResetSidebarOrder returns to the original directory.
func TestResetSidebarOrder(t *testing.T) {
	s := defaultSettings()
	s.moveSidebarEntry(8, 0)
	if reflect.DeepEqual(s.sidebarOrder(), sidebarNavIDs) {
		t.Fatal("the move did not take effect")
	}
	s.resetSidebarOrder()
	if s.SidebarOrder != nil {
		t.Errorf("reset left a saved order: %v", s.SidebarOrder)
	}
	if got := s.sidebarOrder(); !reflect.DeepEqual(got, sidebarNavIDs) {
		t.Errorf("reset gave %v, want %v", got, sidebarNavIDs)
	}
}

// TestDirectoryEntriesFollowOrder: the sidebar and its rail read the
// same ordered list, and hidden pages stay out of the visible one.
func TestDirectoryEntriesFollowOrder(t *testing.T) {
	a := newTestApp()
	a.settings.moveSidebarEntry(8, 0)
	all := a.directoryEntries()
	if all[0].id != "newalbums" {
		t.Errorf("the directory kept its old first page: %s", all[0].id)
	}
	a.settings.SidebarNewAlbums = false
	for _, e := range a.visibleSidebarItems() {
		if e.id == "newalbums" {
			t.Error("a hidden page is still visible in the directory")
		}
	}
	a.settings.SidebarNewAlbums = true
	for _, e := range a.visibleSidebarItems() {
		if e.id == "newalbums" {
			return
		}
	}
	t.Error("a shown page is missing from the directory")
}
