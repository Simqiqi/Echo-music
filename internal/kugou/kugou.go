// Package kugou logs into 酷狗音乐 (KuGou) through the Node sidecar and
// persists the session it produces to the same keychain the NetEase session
// uses. The session is the device cookie carrying the lite token and userid.
package kugou

import (
	"fmt"

	"github.com/simqiqi/simqiqi-music/internal/secretstore"
	"github.com/simqiqi/simqiqi-music/internal/sidecar"
)

const sessionSecret = "kugou-session"

// StartQR begins a QR login and returns the key plus the QR image as a PNG
// base64 string the UI decodes for display.
func StartQR() (key, image string, err error) {
	c, err := sidecar.Shared()
	if err != nil {
		return "", "", err
	}
	return c.StartQR("kugou")
}

// CheckQR polls a QR login; state is waiting|scanned|confirmed|expired.
func CheckQR(key string) (state string, err error) {
	c, err := sidecar.Shared()
	if err != nil {
		return "", err
	}
	return c.CheckQR("kugou", key)
}

// Profile reports who is signed in, or nil when nobody is.
func Profile() (*sidecar.Account, error) {
	c, err := sidecar.Shared()
	if err != nil {
		return nil, err
	}
	return c.Status("kugou")
}

// Save persists the in-memory credentials the bridge holds to the keychain.
func Save() error {
	c, err := sidecar.Shared()
	if err != nil {
		return err
	}
	raw, err := c.Session("kugou")
	if err != nil || len(raw) == 0 {
		return fmt.Errorf("kugou: 没有可保存的登录态")
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
	if err := c.Restore("kugou", raw); err != nil {
		return false, err
	}
	return true, nil
}

// Logout clears the bridge's credentials and the keychain entry. KuGou has
// no remote logout the bridge can reach, so this only forgets local state.
func Logout() error {
	c, err := sidecar.Shared()
	if err == nil {
		c.Logout("kugou")
	}
	secretstore.Delete(sessionSecret)
	return nil
}
