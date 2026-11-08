// Package qishui logs into 汽水音乐 (Qishui / Douyin-family) through the
// Node sidecar, and persists the session the bridge produces to the same
// keychain the NetEase session uses.
package qishui

import (
	"encoding/json"
	"fmt"

	"github.com/simqiqi/simqiqi-music/internal/secretstore"
	"github.com/simqiqi/simqiqi-music/internal/sidecar"
)

const sessionSecret = "qishui-session"

// StartQR begins a QR login and returns the key plus the QR image as a PNG
// base64 string the UI decodes for display.
func StartQR() (key, image string, err error) {
	c, err := sidecar.Shared()
	if err != nil {
		return "", "", err
	}
	return c.StartQR("qishui")
}

// CheckQR polls a QR login; state is waiting|scanned|confirmed|expired.
func CheckQR(key string) (state string, err error) {
	c, err := sidecar.Shared()
	if err != nil {
		return "", err
	}
	return c.CheckQR("qishui", key)
}

// Profile reports who is signed in, or nil when nobody is.
func Profile() (*sidecar.Account, error) {
	c, err := sidecar.Shared()
	if err != nil {
		return nil, err
	}
	return c.Status("qishui")
}

// Save persists the in-memory credentials the bridge holds to the keychain.
func Save() error {
	c, err := sidecar.Shared()
	if err != nil {
		return err
	}
	raw, err := c.Session("qishui")
	if err != nil || len(raw) == 0 {
		return fmt.Errorf("qishui: 没有可保存的登录态")
	}
	return secretstore.Set(sessionSecret, raw)
}

// Restore loads a persisted session back into the bridge, returning whether
// one was found.
func Restore() (bool, error) {
	raw, err := secretstore.Get(sessionSecret)
	if err != nil || len(raw) == 0 {
		return false, err
	}
	c, err := sidecar.Shared()
	if err != nil {
		return false, err
	}
	if err := c.Restore("qishui", raw); err != nil {
		return false, err
	}
	return true, nil
}

// Logout clears the bridge's credentials and the keychain entry.
func Logout() error {
	c, err := sidecar.Shared()
	if err == nil {
		c.Logout("qishui")
	}
	secretstore.Delete(sessionSecret)
	return nil
}

// Credentials just reports the raw session the bridge holds, for diagnostics.
func Credentials() (json.RawMessage, error) {
	c, err := sidecar.Shared()
	if err != nil {
		return nil, err
	}
	return c.Session("qishui")
}
