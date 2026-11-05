package app

import (
	"net/url"
	"os/exec"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
)

// feedbackEmail is where a listener's report or idea goes. It lives only in
// the mailto link: the settings never put the address on screen.
const feedbackEmail = "simqiqi@outlook.com"

// openExternal hands a URL to the system's handler; tests replace it.
// Start, rather than Run, keeps the fork off the UI thread; Wait reaps the
// child without making the click wait for a mail app to come up.
var openExternal = func(raw string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", raw)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", raw)
	default:
		cmd = exec.Command("xdg-open", raw)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

// feedbackMail builds the mail a listener sends: the subject says what kind
// it is, the body carries the version and the system a report needs, and
// neither the subject nor the body has the address in it.
func feedbackMail(kind string) string {
	name, version := mygo.App.Name(), mygo.App.Version()
	subject := "[" + name + "] " + kind + " v" + version
	lines := []string{
		"（在这里写下遇到的问题，或想提的建议）",
		"",
		"复现步骤（如果是问题）：",
		"1. ",
		"2. ",
		"3. ",
		"",
		"期望的结果：",
		"",
		"",
		"—— 以下信息有助于定位，可保留或删去 ——",
		name + " " + version,
		runtime.GOOS + " / " + runtime.GOARCH,
		"",
		"最近的错误（已去掉地址与账号信息）：",
	}
	lines = append(lines, diagnosticLines()...)
	body := strings.Join(lines, "\n")
	q := url.Values{"subject": {subject}, "body": {body}}
	// A mailto query uses %20 for a space, not the + of a form query.
	return "mailto:" + feedbackEmail + "?" + strings.ReplaceAll(q.Encode(), "+", "%20")
}

// writeFeedback opens a report or an idea in the mail app. With no handler,
// it says so rather than showing the address for lack of anything else.
func (a *app) writeFeedback(kind string) {
	if err := openExternal(feedbackMail(kind)); err != nil {
		a.toast = "未能打开邮件应用，请稍后再试"
	}
}
