package app

import (
	"testing"

	"github.com/simqiqi/simqiqi-music/internal/netease"
	"github.com/simqiqi/simqiqi-music/internal/sidecar"
)

// TestAccountInfoPerProvider checks that the account menu shows each
// provider's own account, and "未登录" for the ones that are not signed in.
func TestAccountInfoPerProvider(t *testing.T) {
	a := newTestApp()
	a.profile = &netease.Profile{UserID: 1, Nickname: "网易号", AvatarURL: "https://p1.music.126.net/a.jpg"}
	a.qishui = &sidecar.Account{ID: "2", Name: "汽水号", Avatar: "https://p1.qishui.com/b.png"}

	cases := []struct {
		prov     providerID
		name     string
		avatar   string
		loggedIn bool
	}{
		{provNetease, "网易号", "https://p1.music.126.net/a.jpg", true},
		{provQishui, "汽水号", "https://p1.qishui.com/b.png", true},
	}
	for _, c := range cases {
		name, avatar, loggedIn := a.accountInfo(c.prov)
		if name != c.name || avatar != c.avatar || loggedIn != c.loggedIn {
			t.Errorf("%s: got (%q, %q, %v), want (%q, %q, %v)",
				c.prov.name(), name, avatar, loggedIn, c.name, c.avatar, c.loggedIn)
		}
	}

	// Signed out, each provider reports nobody, without borrowing the
	// other providers' accounts.
	a.profile, a.qishui = nil, nil
	for _, prov := range []providerID{provNetease, provQishui} {
		name, avatar, loggedIn := a.accountInfo(prov)
		if loggedIn || name != "未登录" || avatar != "" {
			t.Errorf("%s signed out: got (%q, %q, %v), want (未登录, \"\", false)",
				prov.name(), name, avatar, loggedIn)
		}
	}
}

// TestTopIdentityFollowsLatestLogin checks that the top bar shows the
// account signed in last, and never goes blank while some account still
// is signed in — the bug where only 汽水音乐 was signed in and the bar
// still read 登录.
func TestTopIdentityFollowsLatestLogin(t *testing.T) {
	a := newTestApp()

	// Nobody: the bar offers 登录.
	if _, _, _, ok := a.topIdentity(); ok {
		t.Fatal("no account signed in: the bar should not show one")
	}

	// Only 汽水 signed in: the bar shows it, though 网易云 (which drives
	// the library) is not.
	a.qishui = &sidecar.Account{ID: "2", Name: "汽水号", Avatar: "https://p1.qishui.com/b.png"}
	a.topProv = provQishui
	prov, name, avatar, ok := a.topIdentity()
	if !ok || prov != provQishui || name != "汽水号" || avatar == "" {
		t.Fatalf("汽水 only: got (%v, %q, %q, %v), want 汽水音乐's account", prov, name, avatar, ok)
	}
}

// TestTopAccountPreference checks that the saved choice wins, that a bridge
// account is preferred over 网易云 by default, and that the choice is
// remembered (the bug where 汽水's avatar came back as 网易云's).
func TestTopAccountPreference(t *testing.T) {
	a := newTestApp()
	a.profile = &netease.Profile{UserID: 1, Nickname: "网易号", AvatarURL: "https://p1.music.126.net/a.jpg"}
	a.qishui = &sidecar.Account{ID: "2", Name: "汽水号", Avatar: "https://p1.qishui.com/b.png"}

	// No choice stored: a bridge account beats 网易云 for the top bar.
	if got := a.resolveTopAccount(); got != provQishui {
		t.Fatalf("default with 汽水+网易云: got %v, want provQishui", got)
	}

	// A value an older build's sign-in left behind is not a choice: the bar
	// still prefers 汽水音乐 rather than keeping 网易云's avatar.
	a.settings.TopAccount = int(provNetease)
	if got := a.resolveTopAccount(); got != provQishui {
		t.Fatalf("stale auto value: got %v, want provQishui", got)
	}

	// The listener picked 网易云: their own choice wins.
	a.settings.TopAccountManual = true
	if got := a.resolveTopAccount(); got != provNetease {
		t.Fatalf("preference 网易云: got %v, want provNetease", got)
	}

	// They switch to 汽水: setTopAccount persists and applies it.
	a.setTopAccount(provQishui)
	if a.settings.TopAccount != int(provQishui) || a.topProv != provQishui {
		t.Fatalf("setTopAccount: stored=%d topProv=%v, want 汽水", a.settings.TopAccount, a.topProv)
	}
	if got := a.resolveTopAccount(); got != provQishui {
		t.Fatalf("preference 汽水: got %v, want provQishui", got)
	}
}

// TestTopAccountLogoutClearsPreference checks that signing out the chosen
// top account drops the saved choice so the bar re-resolves.
func TestTopAccountLogoutClearsPreference(t *testing.T) {
	a := newTestApp()
	a.profile = &netease.Profile{UserID: 1, Nickname: "网易号", AvatarURL: "https://p1.music.126.net/a.jpg"}
	a.qishui = &sidecar.Account{ID: "2", Name: "汽水号", Avatar: "https://p1.qishui.com/b.png"}
	a.settings.TopAccount = int(provQishui)
	a.settings.TopAccountManual = true
	a.finalizeTopAccount()
	if a.topProv != provQishui {
		t.Fatalf("before logout: topProv=%v, want provQishui", a.topProv)
	}
	a.logoutProvider(provQishui)
	if a.settings.TopAccount != int(provNone) {
		t.Fatalf("preference kept after logout: %d, want none", a.settings.TopAccount)
	}
	// 网易云 remains, so it takes the bar.
	if a.topProv != provNetease {
		t.Fatalf("after 汽水 logout: topProv=%v, want provNetease", a.topProv)
	}
}

// TestFollowAccountDoesNotPin checks that signing in moves the top bar but
// leaves no preference behind, so a 汽水音乐 sign-in is not undone on the
// next launch by the 网易云 account that signs in first.
func TestFollowAccountDoesNotPin(t *testing.T) {
	a := newTestApp()
	a.profile = &netease.Profile{UserID: 1, Nickname: "网易号", AvatarURL: "https://p1.music.126.net/a.jpg"}
	a.followAccount(provNetease)
	if a.topProv != provNetease {
		t.Fatalf("after 网易云 sign-in: topProv=%v, want provNetease", a.topProv)
	}
	if a.settings.TopAccount != int(provNone) || a.settings.TopAccountManual {
		t.Fatalf("sign-in pinned the bar: stored=%d manual=%v", a.settings.TopAccount, a.settings.TopAccountManual)
	}

	// Then 汽水音乐 signs in: it should take the bar, avatar and all.
	a.qishui = &sidecar.Account{ID: "2", Name: "汽水号", Avatar: "https://p1.qishui.com/b.png"}
	a.followAccount(provQishui)
	a.finalizeTopAccount()
	prov, name, avatar, ok := a.topIdentity()
	if !ok || prov != provQishui || name != "汽水号" || avatar != "https://p1.qishui.com/b.png" {
		t.Fatalf("after 汽水 sign-in: got (%v, %q, %q, %v), want 汽水音乐's account", prov, name, avatar, ok)
	}

	// A pick made by the listener still wins over a later sign-in.
	a.setTopAccount(provNetease)
	a.qishui = nil
	a.followAccount(provQishui)
	if a.topProv != provNetease {
		t.Fatalf("manual pick overridden: topProv=%v, want provNetease", a.topProv)
	}
}
