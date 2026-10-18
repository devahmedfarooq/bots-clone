package state

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"bots/internal/provider"
	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }
type Run struct {
	ID         string         `json:"id"`
	Session    string         `json:"session"`
	Bot        string         `json:"bot"`
	Version    string         `json:"version"`
	Status     string         `json:"status"`
	Reason     string         `json:"termination_reason"`
	Error      string         `json:"error,omitempty"`
	Output     string         `json:"output"`
	Usage      provider.Usage `json:"usage"`
	Started    time.Time      `json:"started"`
	DurationMS int64          `json:"duration_ms"`
}

func ID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, e := sql.Open("sqlite", path)
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	_, e = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;
 CREATE TABLE IF NOT EXISTS sessions(id TEXT PRIMARY KEY,bot TEXT NOT NULL,version TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS runs(id TEXT PRIMARY KEY,session TEXT NOT NULL,record TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS messages(seq INTEGER PRIMARY KEY AUTOINCREMENT,run TEXT NOT NULL,session TEXT NOT NULL,body TEXT NOT NULL);
 CREATE INDEX IF NOT EXISTS history ON messages(session,seq);
 CREATE TABLE IF NOT EXISTS events(seq INTEGER PRIMARY KEY AUTOINCREMENT,run TEXT NOT NULL,body TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS checkpoints(kind TEXT NOT NULL,id TEXT NOT NULL,body TEXT NOT NULL,PRIMARY KEY(kind,id));`)
	if e != nil {
		db.Close()
		return nil, e
	}
	return &Store{db}, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Session(id, bot, version string) (string, []provider.Message, error) {
	if id == "" {
		id = ID()
		_, e := s.db.Exec("INSERT INTO sessions VALUES(?,?,?)", id, bot, version)
		return id, nil, e
	}
	var b, v string
	if e := s.db.QueryRow("SELECT bot,version FROM sessions WHERE id=?", id).Scan(&b, &v); e != nil {
		return "", nil, e
	}
	if b != bot || v != version {
		return "", nil, errors.New("session belongs to a different bot/version")
	}
	rows, e := s.db.Query("SELECT m.body FROM messages m JOIN runs r ON m.run=r.id WHERE m.session=? AND json_extract(r.record,'$.status')='completed' ORDER BY m.seq", id)
	if e != nil {
		return "", nil, e
	}
	defer rows.Close()
	var messages []provider.Message
	for rows.Next() {
		var data string
		if e = rows.Scan(&data); e != nil {
			return "", nil, e
		}
		var m provider.Message
		if e = json.Unmarshal([]byte(data), &m); e != nil {
			return "", nil, e
		}
		messages = append(messages, m)
	}
	return id, messages, rows.Err()
}

var credential = regexp.MustCompile(`(?i)("?(?:password|api[_-]?key|access[_-]?token|secret)"?\s*[:=]\s*"?)[^\s",}]+`)
var bearer = regexp.MustCompile(`(?i)Bearer\s+[a-z0-9._~+/=-]+|\bsk-[a-zA-Z0-9_-]{8,}`)

func Redact(s string) string {
	s = credential.ReplaceAllString(s, "${1}[REDACTED]")
	return bearer.ReplaceAllString(s, "[REDACTED]")
}
func encoded(v any) ([]byte, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return nil, e
	}
	var data any
	if e = json.Unmarshal(b, &data); e != nil {
		return nil, e
	}
	var clean func(any) any
	clean = func(value any) any {
		switch x := value.(type) {
		case string:
			return Redact(x)
		case map[string]any:
			for k, item := range x {
				if credential.MatchString(k + "=value") {
					x[k] = "[REDACTED]"
				} else {
					x[k] = clean(item)
				}
			}
			return x
		case []any:
			for i, item := range x {
				x[i] = clean(item)
			}
			return x
		default:
			return value
		}
	}
	return json.Marshal(clean(data))
}
func Sanitized(v any) ([]byte, error) { return encoded(v) }
func (s *Store) Save(r Run) error {
	b, e := encoded(r)
	if e != nil {
		return e
	}
	_, e = s.db.Exec("INSERT INTO runs VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET record=excluded.record", r.ID, r.Session, string(b))
	return e
}
func (s *Store) Message(run, session string, m provider.Message) error {
	b, e := encoded(m)
	if e != nil {
		return e
	}
	_, e = s.db.Exec("INSERT INTO messages(run,session,body) VALUES(?,?,?)", run, session, string(b))
	return e
}
func (s *Store) Event(run string, event any) error {
	b, e := encoded(event)
	if e != nil {
		return e
	}
	_, e = s.db.Exec("INSERT INTO events(run,body) VALUES(?,?)", run, string(b))
	return e
}
func (s *Store) Get(id string) (Run, error) {
	var r Run
	var b string
	e := s.db.QueryRow("SELECT record FROM runs WHERE id=?", id).Scan(&b)
	if e != nil {
		return r, e
	}
	e = json.Unmarshal([]byte(b), &r)
	return r, e
}
func (s *Store) ToolEvents(id string) (int, error) {
	var count int
	e := s.db.QueryRow("SELECT count(*) FROM events WHERE run=? AND json_extract(body,'$.result.error') IS NULL", id).Scan(&count)
	return count, e
}
