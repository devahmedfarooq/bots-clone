//go:build !windows

package lock

import (
	"fmt"
	"golang.org/x/sys/unix"
	"os"
)

func Acquire(path string) (func(), error) {
	f, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); e != nil {
		f.Close()
		return nil, fmt.Errorf("another CLI process owns local inference: %w", e)
	}
	return func() { unix.Flock(int(f.Fd()), unix.LOCK_UN); f.Close() }, nil
}
