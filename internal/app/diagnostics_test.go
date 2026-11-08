package app

import (
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func resetDiagnostics() {
	diagMu.Lock()
	diagLog = nil
	diagMu.Unlock()
}

// A diagnostic keeps the shape of a failure but none of what it carried:
// URLs with tokens and account-shaped text are scrubbed, and only the last
// few are kept for a mail.
func TestDiagnosticScrubsAndKeepsRecent(t *testing.T) {
	resetDiagnostics()
	real := diagClock
	diagClock = func() time.Time { return time.Date(2026, 11, 5, 18, 2, 0, 0, time.UTC) }
	defer func() { diagClock = real; resetDiagnostics() }()

	noteDiagnostic("播放", errors.New(`Get "https://cdn.example/song?token=secret": dial tcp 1.2.3.4: timeout; write to a@b.com`))
	lines := diagnosticLines()
	if len(lines) != 1 {
		t.Fatalf("lines: %v", lines)
	}
	if !strings.Contains(lines[0], "[地址]") || !strings.Contains(lines[0], "[账号]") || !strings.Contains(lines[0], "播放") {
		t.Errorf("not scrubbed or missing its source: %q", lines[0])
	}
	if strings.Contains(lines[0], "secret") || strings.Contains(lines[0], "a@b.com") {
		t.Errorf("private text leaked: %q", lines[0])
	}
	for i := 0; i < 15; i++ {
		noteDiagnostic("播放", errors.New("timeout"))
	}
	diagMu.Lock()
	n := len(diagLog)
	diagMu.Unlock()
	if n != diagLimit {
		t.Errorf("kept %d diagnostics, want %d", n, diagLimit)
	}
	if got := len(diagnosticLines()); got != 6 {
		t.Errorf("the mail got %d lines, want 6", got)
	}
}

// A feedback mail carries the recent failures, and still never spells the
// address in what the listener can read.
func TestFeedbackMailCarriesDiagnostics(t *testing.T) {
	resetDiagnostics()
	defer resetDiagnostics()
	noteDiagnostic("播放", errors.New("dial tcp: timeout"))

	raw := feedbackMail("问题反馈")
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	body := u.Query().Get("body")
	if !strings.Contains(body, "最近的错误") || !strings.Contains(body, "dial tcp: timeout") {
		t.Errorf("the mail has no diagnostics: %q", body)
	}
	if strings.Contains(body, feedbackEmail) {
		t.Error("the address is written into the mail's body")
	}
}

// The five probes are asked together and keep their own results.
func TestRunChecksGathersProbes(t *testing.T) {
	pl, pa, ps, pf, pq := probeLogin, probeAPI, probeSong, probeFeed, probeQishui
	defer func() { probeLogin, probeAPI, probeSong, probeFeed, probeQishui = pl, pa, ps, pf, pq }()
	probeLogin = func(*app) checkResult { return checkResult{"登录状态", true, "已登录"} }
	probeAPI = func() checkResult { return checkResult{"网易云接口", false, "timeout"} }
	probeSong = func(*app, int64) checkResult { return checkResult{"播放地址", true, "已跳过"} }
	probeFeed = func() checkResult { return checkResult{"更新镜像", true, "可达"} }
	probeQishui = func(*app) checkResult { return checkResult{"汽水音乐", true, "未登录，已跳过"} }

	a := newTestApp()
	got := a.runChecks()
	if len(got) != 5 {
		t.Fatalf("got %d results: %+v", len(got), got)
	}
	want := []string{"登录状态", "网易云接口", "播放地址", "更新镜像", "汽水音乐"}
	for i := range want {
		if got[i].name != want[i] {
			t.Errorf("result %d is %q, want %q", i, got[i].name, want[i])
		}
	}
	if got[1].ok {
		t.Error("the failed API probe was reported as working")
	}
}

// Only what failed goes into the diagnostics that a feedback mail carries.
func TestFinishChecksNotesFailures(t *testing.T) {
	resetDiagnostics()
	defer resetDiagnostics()
	a := newTestApp()
	a.checking = true
	a.finishChecks([]checkResult{
		{"登录状态", true, "已登录"},
		{"网易云接口", false, "timeout"},
	})
	if a.checking || len(a.checks) != 2 {
		t.Errorf("checking=%v checks=%+v", a.checking, a.checks)
	}
	lines := diagnosticLines()
	if len(lines) != 1 || !strings.Contains(lines[0], "连接自检 · 网易云接口") {
		t.Errorf("diagnostics: %v", lines)
	}
}

// The settings' check button starts a check, with the probes answered in
// the test so nothing goes out to the network.
func TestConnectionCheckButton(t *testing.T) {
	pl, pa, ps, pf, pq := probeLogin, probeAPI, probeSong, probeFeed, probeQishui
	defer func() {
		// startChecks has a goroutine of its own: let it finish before
		// the probes it reads are put back.
		time.Sleep(20 * time.Millisecond)
		probeLogin, probeAPI, probeSong, probeFeed, probeQishui = pl, pa, ps, pf, pq
	}()
	ok := func(name string) func() checkResult {
		return func() checkResult { return checkResult{name, true, "可达"} }
	}
	probeLogin = func(*app) checkResult { return checkResult{"登录状态", true, "已登录"} }
	probeAPI, probeFeed = ok("网易云接口"), ok("更新镜像")
	probeSong = func(*app, int64) checkResult { return checkResult{"播放地址", true, "已跳过"} }
	probeQishui = func(*app) checkResult { return checkResult{"汽水音乐", true, "未登录，已跳过"} }

	a := newTestApp()
	a.settings = defaultSettings()
	tt := ui.NewTester(func(c *ui.Context) { a.settingsTestView(c) }, 620, 1600)
	tt.Frame()
	if err := tt.Click("开始检查"); err != nil {
		t.Fatalf("click: %v", err)
	}
	tt.Frame()
	if !a.checking {
		t.Error("the button did not start a connection check")
	}
}
