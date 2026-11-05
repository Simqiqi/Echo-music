package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

// latestRelease asks GitHub for the newest release of the app: the tag it
// wears and the page that tells of it. Tests replace it, so that no test
// asks the network.
var latestRelease = func() (tag, page string, err error) {
	c := &http.Client{Timeout: 12 * time.Second}
	res, err := c.Get("https://api.github.com/repos/Simqiqi/simqiqi-music/releases/latest")
	if err != nil {
		return "", "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("GitHub answered %s", res.Status)
	}
	var r struct {
		Tag string `json:"tag_name"`
		URL string `json:"html_url"`
	}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return "", "", err
	}
	return r.Tag, r.URL, nil
}

// updatePeek is the newest release the app has heard of. The top bar wears
// a dot while that release is newer than the one running; the updater's own
// window still opens only when the listener asks for it.
type updatePeek struct {
	at   time.Time // when it was heard of
	tag  string    // the newest tag, "v0.3.4"
	page string    // where to read of it
}

// appVersion is the version running, which tests set: a build that cannot
// say its version never claims to be old.
var appVersion = func() string { return mygo.App.Version() }

// out reports whether what was heard of is newer than this build.
func (u updatePeek) out() bool {
	now := appVersion()
	return u.tag != "" && now != "" && versionLess(now, u.tag)
}

// watchUpdates hears of the newest release soon after the app opens, and
// then every six hours.
func (a *app) watchUpdates() {
	go func() {
		time.Sleep(8 * time.Second)
		for {
			a.checkLatest()
			time.Sleep(6 * time.Hour)
		}
	}()
}

// checkLatest asks for the newest release and keeps it for the top bar. A
// failure is nothing to say: the app works without knowing of releases.
func (a *app) checkLatest() {
	tag, page, err := latestRelease()
	if err != nil {
		return
	}
	a.post(func() {
		a.updatePeek = updatePeek{at: time.Now(), tag: tag, page: page}
	})
}

// versionParts reads "v0.3.3" as its numbers: what a pre-release says ahead
// of its dash is its version.
func versionParts(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	v = strings.SplitN(v, "-", 2)[0]
	var out [3]int
	for i, s := range strings.Split(v, ".") {
		if i >= len(out) {
			break
		}
		out[i], _ = strconv.Atoi(s)
	}
	return out
}

// versionLess reports whether now is an older version than tag.
func versionLess(now, tag string) bool {
	a, b := versionParts(now), versionParts(tag)
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}
