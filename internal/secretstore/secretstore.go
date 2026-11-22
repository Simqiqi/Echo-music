// Package secretstore keeps small secrets (a provider's login session) in
// the same place NetEase's session lives: the system keychain in a release
// build, and a private file in the user-data dir as well — because every
// rebuild re-signs the app, and the keychain turns away what another
// signature stored, which used to sign the listener out on each rebuild.
//
// 汽水音乐's session is larger than what the keychain keeps per item
// (mygo.MaxSecretSize, 2560 bytes: it carries a device certificate), so a
// secret that does not fit goes to a private file of the user-data dir
// instead — where the app's other session data lives, owner-only. Small
// ones still go to the keychain, and a read looks in both.
package secretstore

import (
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

// keyRoom is the most a secret may take before it goes to a file: the
// keychain's limit, less a margin, since it counts the value in UTF-8.
const keyRoom = mygo.MaxSecretSize - 64

// Get reads secret name, returning (nil, nil) when it is not stored. The
// keychain is asked first; anything it refuses falls through to the file.
// A rebuilt app is the case that matters: its signature no longer matches
// what stored the secret, so the keychain turns it away, and before this
// the listener was signed out by a mere rebuild.
func Get(name string) ([]byte, error) {
	if !mygo.IsDev() {
		if b, err := mygo.Secrets.Get(name); err == nil {
			return b, nil
		}
	}
	return readFile(name)
}

// Set writes secret name, overwriting any previous value: the keychain,
// for a release build to keep it in, and always the file beside the app's
// other session data (owner-only) — so that a later build the keychain
// turns away can still read it. NetEase's own session lives in that file
// too, so this is the same bargain the app already makes.
func Set(name string, b []byte) error {
	if !mygo.IsDev() && len(b) <= keyRoom {
		_ = mygo.Secrets.Set(name, b)
	}
	return writeFile(name, b)
}

// Delete removes secret name. Missing secrets are ignored.
func Delete(name string) {
	if !mygo.IsDev() {
		_ = mygo.Secrets.Delete(name)
	}
	removeFile(name)
}

func readFile(name string) ([]byte, error) {
	path, err := file(name)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return b, nil
}

func writeFile(name string, b []byte) error {
	path, err := file(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path+".part", b, 0o600); err != nil {
		return err
	}
	return os.Rename(path+".part", path)
}

func removeFile(name string) {
	if path, err := file(name); err == nil {
		os.Remove(path)
	}
}

func file(name string) (string, error) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "session-"+name), nil
}
