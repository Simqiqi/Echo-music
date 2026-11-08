// Package secretstore keeps small secrets (a provider's login session) in
// the same place NetEase's session lives: the system keychain in a release
// build, a private file in the user-data dir while developing. Development
// uses a file because every rebuild re-signs the app and the keychain would
// prompt before each one read it.
//
// 汽水音乐's session is larger than what the keychain keeps per item
// (mygo.MaxSecretSize, 2560 bytes: it carries a device certificate), so a
// secret that does not fit goes to a private file of the user-data dir
// instead — where the app's other session data lives, owner-only. Small
// ones still go to the keychain, and a read looks in both.
package secretstore

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

// keyRoom is the most a secret may take before it goes to a file: the
// keychain's limit, less a margin, since it counts the value in UTF-8.
const keyRoom = mygo.MaxSecretSize - 64

// Get reads secret name, returning (nil, nil) when it is not stored.
func Get(name string) ([]byte, error) {
	if !mygo.IsDev() {
		b, err := mygo.Secrets.Get(name)
		switch {
		case err == nil:
			return b, nil
		case !errors.Is(err, mygo.ErrSecretNotFound):
			return nil, err
		}
	}
	return readFile(name)
}

// Set writes secret name, overwriting any previous value.
func Set(name string, b []byte) error {
	if !mygo.IsDev() && len(b) <= keyRoom {
		if err := mygo.Secrets.Set(name, b); err == nil {
			removeFile(name)
			return nil
		}
		// The keychain refused it for another reason (a locked or
		// unavailable store): the file keeps the listener signed in too.
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
