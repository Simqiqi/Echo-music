package app

import (
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/simqiqi/simqiqi-music/internal/netease"
)

// A short, private record of what went wrong. It goes into a feedback mail
// so a report says what the app saw, without carrying anything of the
// listener's: no URLs with tokens, no addresses, no long payloads.
type diagnostic struct {
	at     time.Time
	where  string
	detail string
}

const diagLimit = 12

var (
	diagMu    sync.Mutex
	diagLog   []diagnostic
	diagClock = time.Now

	diagURL   = regexp.MustCompile(`https?://[^\s"']+`)
	diagEmail = regexp.MustCompile(`\S+@\S+`)
)

// cleanDiagnostic keeps the shape of an error but not what it might carry.
func cleanDiagnostic(err error) string {
	if err == nil {
		return "未知错误"
	}
	s := diagURL.ReplaceAllString(err.Error(), "[地址]")
	s = diagEmail.ReplaceAllString(s, "[账号]")
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 120 {
		s = string(r[:120]) + "…"
	}
	return s
}

// noteDiagnostic keeps a failure, dropping the oldest past the limit.
func noteDiagnostic(where string, err error) {
	diagMu.Lock()
	defer diagMu.Unlock()
	diagLog = append(diagLog, diagnostic{at: diagClock(), where: where, detail: cleanDiagnostic(err)})
	if len(diagLog) > diagLimit {
		diagLog = diagLog[len(diagLog)-diagLimit:]
	}
}

// lastDiagnostic is the latest failure, if one happened within the span
// given: what the top bar's lamp turns amber for.
func lastDiagnostic(within time.Duration) (diagnostic, bool) {
	diagMu.Lock()
	defer diagMu.Unlock()
	if len(diagLog) == 0 {
		return diagnostic{}, false
	}
	d := diagLog[len(diagLog)-1]
	if diagClock().Sub(d.at) > within {
		return diagnostic{}, false
	}
	return d, true
}

// diagnosticLines are the latest failures, oldest first, for a mail's body.
func diagnosticLines() []string {
	diagMu.Lock()
	defer diagMu.Unlock()
	if len(diagLog) == 0 {
		return []string{"（最近没有记录到错误）"}
	}
	from := max(0, len(diagLog)-6)
	var out []string
	for _, d := range diagLog[from:] {
		out = append(out, fmt.Sprintf("%s · %s · %s", d.at.Format("01-02 15:04"), d.where, d.detail))
	}
	return out
}

// checkResult is one leg of a connection check, and what it found.
type checkResult struct {
	name   string
	ok     bool
	detail string
}

// The probes are kept apart so a test can answer them without a network.
var (
	probeLogin = func(a *app) checkResult {
		if a.profile != nil {
			return checkResult{"登录状态", true, "已登录网易云音乐"}
		}
		return checkResult{"登录状态", false, "未登录，每日推荐与歌单不可用"}
	}
	probeAPI = func() checkResult {
		if _, err := netease.Fetch("https://music.163.com/api/song/lyric?id=0&lv=1"); err != nil {
			return checkResult{"网易云接口", false, cleanDiagnostic(err)}
		}
		return checkResult{"网易云接口", true, "接口可达"}
	}
	probeSong = func(id int64) checkResult {
		if id == 0 {
			return checkResult{"播放地址", true, "未在播放，已跳过"}
		}
		if _, err := netease.SongURL(id, "standard"); err != nil {
			return checkResult{"播放地址", false, cleanDiagnostic(err)}
		}
		return checkResult{"播放地址", true, "取到了播放地址"}
	}
	probeFeed = func() checkResult {
		u := fmt.Sprintf("https://cdn.jsdelivr.net/gh/Simqiqi/simqiqi-music@updates/update-%s-%s.json",
			runtime.GOOS, runtime.GOARCH)
		if _, err := netease.Fetch(u); err != nil {
			return checkResult{"更新镜像", false, cleanDiagnostic(err)}
		}
		return checkResult{"更新镜像", true, "更新清单可达"}
	}
)

// runChecks asks every probe at once, so the slowest one sets the wait.
func (a *app) runChecks() []checkResult {
	var id int64
	if tr := a.pl.Current(); tr != nil {
		id = tr.ID
	}
	out := make([]checkResult, 4)
	var wg sync.WaitGroup
	run := func(i int, fn func() checkResult) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = fn()
		}()
	}
	run(0, func() checkResult { return probeLogin(a) })
	run(1, probeAPI)
	run(2, func() checkResult { return probeSong(id) })
	run(3, probeFeed)
	wg.Wait()
	return out
}

// finishChecks keeps what the check found, and remembers the failures for
// the next feedback mail.
func (a *app) finishChecks(results []checkResult) {
	a.checking, a.checks = false, results
	for _, r := range results {
		if !r.ok {
			noteDiagnostic("连接自检 · "+r.name, errors.New(r.detail))
		}
	}
}

// startChecks runs the check off the UI thread and brings its result back.
func (a *app) startChecks() {
	if a.checking {
		return
	}
	a.checking, a.checks = true, nil
	a.redraw()
	go func() {
		results := a.runChecks()
		a.post(func() {
			a.finishChecks(results)
			a.redraw()
		})
	}()
}
