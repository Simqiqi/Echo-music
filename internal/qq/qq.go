// Package qq logs into QQ 音乐 through the Node sidecar and persists the
// session it produces to the same keychain the other providers use. The
// session is the y.qq.com cookie (uin + qm_keyst/qqmusic_key) the bridge
// obtained by scanning a QR code — with QQ, or with WeChat.
package qq

import (
	"fmt"

	"github.com/simqiqi/simqiqi-music/internal/secretstore"
	"github.com/simqiqi/simqiqi-music/internal/sidecar"
)

const sessionSecret = "qq-session"

// The two ways into a QQ 音乐 account. They end in the same session; the
// difference is only which QR the listener is shown.
const (
	LoginQQ     = "qq" // 用 QQ App 扫码
	LoginWeChat = "wx" // 用微信扫码
)

// StartQR begins a QR login and returns the key plus the QR image as a PNG
// base64 string the UI decodes for display. mode picks the channel: LoginQQ
// (the default when empty) or LoginWeChat.
func StartQR(mode string) (key, image string, err error) {
	c, err := sidecar.Shared()
	if err != nil {
		return "", "", err
	}
	if mode != LoginWeChat {
		mode = LoginQQ
	}
	return c.StartQRMode("qq", mode)
}

// CheckQR polls a QR login; state is waiting|scanned|confirmed|expired. The
// key carries the channel, so no mode is needed here.
func CheckQR(key string) (state string, err error) {
	c, err := sidecar.Shared()
	if err != nil {
		return "", err
	}
	return c.CheckQR("qq", key)
}

// Profile reports who is signed in, or nil when nobody is.
func Profile() (*sidecar.Account, error) {
	c, err := sidecar.Shared()
	if err != nil {
		return nil, err
	}
	return c.Status("qq")
}

// Save persists the in-memory credentials the bridge holds to the keychain.
func Save() error {
	c, err := sidecar.Shared()
	if err != nil {
		return err
	}
	raw, err := c.Session("qq")
	if err != nil || len(raw) == 0 {
		return fmt.Errorf("qq: 没有可保存的登录态")
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
	if err := c.Restore("qq", raw); err != nil {
		return false, err
	}
	return true, nil
}

// Logout clears the bridge's credentials and the keychain entry. The
// y.qq.com cookie lives locally, so forgetting it is all there is to do.
func Logout() error {
	c, err := sidecar.Shared()
	if err == nil {
		c.Logout("qq")
	}
	secretstore.Delete(sessionSecret)
	return nil
}
