package state

import (
	"path/filepath"
	"testing"
	"time"
)

func TestRetentionProtectsRunningAndRequested(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, r := range []Run{{ID: "old", Status: "completed", Started: time.Now().Add(-48 * time.Hour)}, {ID: "keep", Status: "completed", Started: time.Now().Add(-48 * time.Hour)}, {ID: "active", Status: "running", Started: time.Now().Add(-48 * time.Hour)}, {ID: "new", Status: "completed", Started: time.Now()}} {
		if err = s.Save(r); err != nil {
			t.Fatal(err)
		}
	}
	ids, err := s.RetentionPlan(time.Now().Add(-24*time.Hour), map[string]bool{"keep": true})
	if err != nil || len(ids) != 1 || ids[0] != "old" {
		t.Fatal(ids, err)
	}
	if err = s.DeleteRuns(append(ids, "active")); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Get("old"); err == nil {
		t.Fatal("old record retained")
	}
	for _, id := range []string{"keep", "active", "new"} {
		if _, err = s.Get(id); err != nil {
			t.Fatal(id, err)
		}
	}
}
