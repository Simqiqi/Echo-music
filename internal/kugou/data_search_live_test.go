package kugou

import (
	"os"
	"testing"
)

// 酷狗的搜索不需要登录，LIVE=1 时真连一次，确认字段映射没退。
func TestBridgeSearchFindsSongs(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip("LIVE is not set: skipping the bridge and 酷狗's search")
	}
	items, _, _, err := Search("周杰伦", 5, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("search found nothing")
	}
	first := items[0]
	if first.ID == "" || first.Name == "" || first.Hash == "" {
		t.Errorf("a result came back without id/name/hash: %+v", first)
	}
}
