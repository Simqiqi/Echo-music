package app

import (
	"errors"
	"net/url"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Both feedback buttons open a mail to the developer, each with a subject
// of its own; the settings keep the address out of sight.
func TestFeedbackButtonsOpenMail(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	for _, tc := range []struct{ button, kind string }{
		{"报告问题", "问题反馈"},
		{"提建议", "功能建议"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			var opened string
			real := openExternal
			openExternal = func(raw string) error { opened = raw; return nil }
			defer func() { openExternal = real }()

			tt := ui.NewTester(func(c *ui.Context) { a.settingsTestView(c) }, 620, 1400)
			tt.Frame()
			if err := tt.Click(tc.button); err != nil {
				t.Fatalf("click on %q: %v", tc.button, err)
			}
			tt.Advance(150 * time.Millisecond)
			tt.Frame()

			u, err := url.Parse(opened)
			if err != nil {
				t.Fatalf("opened %q: %v", opened, err)
			}
			if u.Scheme != "mailto" || u.Opaque != feedbackEmail {
				t.Errorf("opened %q, want a mail to the developer", opened)
			}
			q := u.Query()
			if !strings.Contains(q.Get("subject"), tc.kind) {
				t.Errorf("subject %q misses %q", q.Get("subject"), tc.kind)
			}
			if !strings.Contains(q.Get("body"), "复现步骤") || !strings.Contains(q.Get("body"), runtime.GOOS) {
				t.Errorf("body misses the report template or the system: %q", q.Get("body"))
			}
			if strings.Contains(q.Get("subject"), feedbackEmail) || strings.Contains(q.Get("body"), feedbackEmail) {
				t.Error("the address is written into what the listener sees")
			}
		})
	}
}

// A mailto link spells its spaces as %20; a form query's + would reach the
// mail app as a literal plus.
func TestFeedbackMailEscapesSpaces(t *testing.T) {
	was := mygo.App.Version()
	mygo.App.SetVersion("9.9.9")
	defer mygo.App.SetVersion(was)

	raw := feedbackMail("问题反馈")
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parsing %q: %v", raw, err)
	}
	if strings.Contains(raw, "+") {
		t.Errorf("form escaping leaked into %q", raw)
	}
	if !strings.Contains(u.Query().Get("subject"), "9.9.9") {
		t.Error("the subject does not say which version it came from")
	}
	if !strings.Contains(u.Query().Get("subject"), " ") {
		t.Error("the subject lost its spaces")
	}
}

// Without a mail handler the app says so, and still does not put the
// address on screen.
func TestFeedbackWithoutMailApp(t *testing.T) {
	a := newTestApp()
	real := openExternal
	openExternal = func(string) error { return errors.New("no handler") }
	defer func() { openExternal = real }()

	a.writeFeedback("问题反馈")
	if a.toast == "" {
		t.Error("no word when the mail app will not open")
	}
	if strings.Contains(a.toast, feedbackEmail) {
		t.Error("the address is shown in the failure message")
	}
}
