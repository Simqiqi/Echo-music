package app

import "testing"

// TestSidebarAutoCollapseDefaultOn checks the new setting: off means the
// directory stays open, on (and the default, nil) means it folds to its
// rail as the pointer leaves.
func TestSidebarAutoCollapseDefaultOn(t *testing.T) {
	var s settings
	if !s.sidebarAutoCollapse() {
		t.Fatal("default should be on (nil means yes)")
	}
	off := false
	s.SidebarAutoCollapse = &off
	if s.sidebarAutoCollapse() {
		t.Fatal("explicit off should disable auto-collapse")
	}
	on := true
	s.SidebarAutoCollapse = &on
	if !s.sidebarAutoCollapse() {
		t.Fatal("explicit on should enable auto-collapse")
	}
}
