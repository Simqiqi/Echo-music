package netease

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/url"
	"strings"
)

// The web player at music.163.com calls its API with the payload sealed
// twice in AES, and the key that unseals it sealed in RSA. The eapi is what
// the desktop client speaks, and both answer the same about a song — but a
// run of calls to the eapi comes back -460, "检测到您的网络环境存在风险，请稍后再试",
// which a listener hears as a song that will not load now and does a moment
// later. The web API does not answer that to the same run, so a song's
// address is asked for there first, and the eapi kept for what it says.

const (
	weapiBase  = "https://music.163.com"
	weapiNonce = "0CoJUm6Qyw8W8jud" // the key the web page carries, in the clear
	weapiIV    = "0102030405060708"
)

// The RSA key the web page seals a session's secret key with: 0x10001,
// and the modulus every client shares.
const weapiModulus = "00e0b509f6259df8642dbc35662901477df22677ec152b5ff68ace615bb7b725" +
	"152b3ab17a876aea8a5aa76d2e417629ec4ee341f56135fccf695280104e0312ecbd" +
	"a92557c93870114af6c9d05c4f7f0c3685b7a46bee255932575cce10b424d813cfe48" +
	"75d3e82047b97ddef52741d546b8e289dc6935b3ece0462db0a22b8e7"

const weapiExponent = 0x010001

// weapiAlphabet is what a secret key is drawn from: letters and digits, as
// the page draws it. The key is reversed and sealed as bytes, so what it
// holds matters less than that it is 16 of them.
const weapiAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// weapiPost calls path (as "/api/song/enhance/player/url/v1") through the
// web API with data, and decodes the JSON it answers into out. A code other
// than 200 is left to the caller: it is an answer about the song, not a
// failed call. The session's cookies ride along in the jar, which the web
// API reads as the eapi does.
func weapiPost(path string, data map[string]any, out any) error {
	body := make(map[string]any, len(data)+1)
	for k, v := range data {
		body[k] = v
	}
	body["csrf_token"] = csrfToken()
	text, err := json.Marshal(body)
	if err != nil {
		return err
	}
	secret, err := weapiSecret()
	if err != nil {
		return err
	}
	params, err := weapiParams(string(text), secret)
	if err != nil {
		return err
	}
	form := url.Values{"params": {params}, "encSecKey": {weapiKey(secret)}}
	req, err := http.NewRequest("POST", weapiBase+"/weapi/"+strings.TrimPrefix(path, "/api/"), strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Referer", weapiBase+"/")
	req.Header.Set("Origin", weapiBase)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Through the jar: the session's MUSIC_U is what makes VIP songs play.
	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return json.NewDecoder(res.Body).Decode(out)
}

// csrfToken is the session's token, which the web API wants beside the
// cookies that carry it.
func csrfToken() string {
	for _, c := range httpClient.Jar.Cookies(siteURL) {
		if c.Name == "__csrf" {
			return c.Value
		}
	}
	return ""
}

// weapiSecret makes the session's own 16-character key.
func weapiSecret() ([]byte, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	for i := range b {
		b[i] = weapiAlphabet[int(b[i])%len(weapiAlphabet)]
	}
	return b, nil
}

// weapiParams seals text with the page's key and then with the session's:
// the second seal is of the first's base64, as the page does it.
func weapiParams(text string, secret []byte) (string, error) {
	once, err := weapiSeal([]byte(text), []byte(weapiNonce))
	if err != nil {
		return "", err
	}
	twice, err := weapiSeal([]byte(base64.StdEncoding.EncodeToString(once)), secret)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(twice), nil
}

// weapiSeal seals plain in AES-CBC, padded to the block as the page pads it.
func weapiSeal(plain, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	pad := aes.BlockSize - len(plain)%aes.BlockSize
	buf := make([]byte, len(plain)+pad)
	copy(buf, plain)
	for i := len(plain); i < len(buf); i++ {
		buf[i] = byte(pad)
	}
	out := make([]byte, len(buf))
	cipher.NewCBCEncrypter(block, []byte(weapiIV)).CryptBlocks(out, buf)
	return out, nil
}

// weapiKey seals the session's key in RSA, reversed and in hex, as the page
// does: the API reads it back the other way round.
func weapiKey(secret []byte) string {
	rev := make([]byte, len(secret))
	for i := range secret {
		rev[i] = secret[len(secret)-1-i]
	}
	n, ok := new(big.Int).SetString(weapiModulus, 16)
	if !ok {
		return ""
	}
	m := new(big.Int).SetBytes(rev)
	m.Exp(m, big.NewInt(weapiExponent), n)
	hex := m.Text(16)
	if len(hex) < 256 {
		hex = strings.Repeat("0", 256-len(hex)) + hex
	}
	return hex
}
