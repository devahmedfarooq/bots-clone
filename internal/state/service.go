package state

import (
	"encoding/json"
	"time"
)

const dispatchRuns = `SELECT json_extract(body,'$.run.id') AS run_id FROM checkpoints WHERE kind='job' AND coalesce(json_extract(body,'$.dispatch_key'),'')<>'' AND json_extract(body,'$.run.id') IS NOT NULL`

func (s *Store) Runs(limit int) ([]Run, error) {
	rows, err := s.db.Query("SELECT record FROM runs ORDER BY rowid DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Run{}
	for rows.Next() {
		var data string
		var r Run
		if err = rows.Scan(&data); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(data), &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) Trace(id string) ([]json.RawMessage, error) {
	rows, err := s.db.Query("SELECT body FROM events WHERE run=? ORDER BY seq", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var body string
		if err = rows.Scan(&body); err != nil {
			return nil, err
		}
		out = append(out, json.RawMessage(body))
	}
	return out, rows.Err()
}

func (s *Store) Checkpoint(kind, id string, v any) error {
	b, err := encoded(v)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("INSERT INTO checkpoints(kind,id,body) VALUES(?,?,?) ON CONFLICT(kind,id) DO UPDATE SET body=excluded.body", kind, id, string(b))
	return err
}

func (s *Store) Checkpoints(kind string) ([]json.RawMessage, error) {
	rows, err := s.db.Query("SELECT body FROM checkpoints WHERE kind=? ORDER BY rowid", kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var body string
		if err = rows.Scan(&body); err != nil {
			return nil, err
		}
		out = append(out, json.RawMessage(body))
	}
	return out, rows.Err()
}

func (s *Store) Recover() error {
	_, err := s.db.Exec(`UPDATE runs SET record=json_set(record,'$.status','interrupted','$.termination_reason','restart_uncertain','$.error','Service stopped; tool actions may have completed. Do not replay automatically.') WHERE json_extract(record,'$.status')='running'`)
	return err
}

func (s *Store) RetentionPlan(before time.Time, keep map[string]bool) ([]string, error) {
	rows, err := s.db.Query("SELECT id,record FROM runs WHERE id NOT IN (" + dispatchRuns + ")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id, body string
		var r Run
		if err = rows.Scan(&id, &body); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(body), &r); err != nil {
			return nil, err
		}
		if r.Status != "running" && r.Started.Before(before) && !keep[id] {
			out = append(out, id)
		}
	}
	return out, rows.Err()
}

func (s *Store) DeleteRuns(ids []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		var retained int
		if err = tx.QueryRow("SELECT count(*) FROM ("+dispatchRuns+") WHERE run_id=?", id).Scan(&retained); err != nil {
			return err
		}
		if retained > 0 {
			continue
		}
		var status string
		if err = tx.QueryRow("SELECT json_extract(record,'$.status') FROM runs WHERE id=?", id).Scan(&status); err != nil {
			return err
		}
		if status == "running" {
			continue
		}
		if _, err = tx.Exec("DELETE FROM checkpoints WHERE kind='job' AND json_extract(body,'$.run.id')=? AND json_extract(body,'$.status') NOT IN ('queued','running')", id); err != nil {
			return err
		}
		for _, table := range []string{"messages", "events", "runs"} {
			key := "run"
			if table == "runs" {
				key = "id"
			}
			if _, err = tx.Exec("DELETE FROM "+table+" WHERE "+key+"=?", id); err != nil {
				return err
			}
		}
	}
	_, err = tx.Exec("DELETE FROM sessions WHERE id NOT IN (SELECT session FROM runs)")
	if err != nil {
		return err
	}
	return tx.Commit()
}
