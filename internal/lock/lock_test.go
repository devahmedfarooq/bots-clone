package lock

import (
	"path/filepath"
	"testing"
)

func TestExclusiveOwnershipAndRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inference.lock")
	release, e := Acquire(path)
	if e != nil {
		t.Fatal(e)
	}
	if second, e := Acquire(path); e == nil {
		second()
		t.Fatal("duplicate owner allowed")
	}
	release()
	release, e = Acquire(path)
	if e != nil {
		t.Fatal("lock not released", e)
	}
	release()
}
