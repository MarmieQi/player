package storage

import (
	"fmt"
	"github.com/gofrs/flock"
	"os"
	"path/filepath"
)

// The server and offline rebuild must never own the same database concurrently.
func ProcessLock(path string) (func(), error) {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return nil, e
	}
	lock := flock.New(path + ".lock")
	ok, e := lock.TryLock()
	if e != nil {
		_ = lock.Close()
		return nil, e
	}
	if !ok {
		_ = lock.Close()
		return nil, fmt.Errorf("database is already owned by another server or rebuild process")
	}
	return func() { _ = lock.Unlock(); _ = lock.Close() }, nil
}
