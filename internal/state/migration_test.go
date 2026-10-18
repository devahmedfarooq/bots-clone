package state

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestMetadataUpgradeBacksUpAndPreservesRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TABLE sessions(id TEXT PRIMARY KEY,bot TEXT NOT NULL,version TEXT NOT NULL); INSERT INTO sessions VALUES('session','bot','1.0.0'); PRAGMA user_version=1;`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, _, err = s.Session("session", "bot", "1.0.0"); err != nil {
		t.Fatal("session lost", err)
	}
	backups, err := filepath.Glob(path + ".backup-*.db")
	if err != nil || len(backups) != 1 {
		t.Fatal(backups, err)
	}
	backup, err := sql.Open("sqlite", backups[0])
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	var count int
	if err = backup.QueryRow("SELECT count(*) FROM sessions").Scan(&count); err != nil || count != 1 {
		t.Fatal("backup invalid", count, err)
	}
	var schema int
	if err = s.db.QueryRow("PRAGMA user_version").Scan(&schema); err != nil || schema != 2 {
		t.Fatal(schema, err)
	}
}
func TestNewerMetadataSchemaRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	db.Exec("PRAGMA user_version=99")
	db.Close()
	if s, err := Open(path); err == nil {
		s.Close()
		t.Fatal("newer schema overwritten")
	}
}
