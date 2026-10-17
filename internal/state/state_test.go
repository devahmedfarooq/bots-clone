package state

import (
	"bots/internal/provider"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionOwnershipAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata.db")
	s, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	session, _, e := s.Session("", "one", "1.0.0")
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = s.Session(session, "two", "1.0.0"); e == nil {
		t.Fatal("cross-bot history allowed")
	}
	if _, _, e = s.Session(session, "one", "2.0.0"); e == nil {
		t.Fatal("cross-version history allowed")
	}
	r := Run{ID: ID(), Session: session, Bot: "one", Version: "1.0.0", Status: "completed"}
	if e = s.Save(r); e != nil {
		t.Fatal(e)
	}
	if e = s.Message(r.ID, session, provider.Message{Role: "user", Content: `{"api_key":"fixture-secret"} password=hunter2 Bearer abc123`}); e != nil {
		t.Fatal(e)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	s, e = Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	_, messages, e := s.Session(session, "one", "1.0.0")
	if e != nil || len(messages) != 1 {
		t.Fatal(messages, e)
	}
	if strings.Contains(messages[0].Content, "fixture-secret") || strings.Contains(messages[0].Content, "hunter2") || strings.Contains(messages[0].Content, "abc123") {
		t.Fatal("secret persisted", messages)
	}
	failed := Run{ID: ID(), Session: session, Status: "failed"}
	if e = s.Save(failed); e != nil {
		t.Fatal(e)
	}
	s.Message(failed.ID, session, provider.Message{Role: "user", Content: "failed input"})
	_, messages, e = s.Session(session, "one", "1.0.0")
	if e != nil || len(messages) != 1 {
		t.Fatal("failed messages resumed", messages, e)
	}
}
