package service

import (
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"time"

	"bots/internal/application"
	"bots/internal/config"
	"bots/internal/state"
)

type Server struct {
	Catalog application.Catalog
	Queue   *Queue
	Store   *state.Store
}

func respond(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func problem(w http.ResponseWriter, err error) {
	respond(w, http.StatusBadRequest, map[string]string{"error": state.Redact(err.Error())})
}

func body(w http.ResponseWriter, r *http.Request, v any) error {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, (1<<20)+4096))
	if err != nil {
		return err
	}
	return config.Decode(data, v)
}

func (s *Server) Handler() http.Handler {
	m := http.NewServeMux()
	m.Handle("GET /", dashboard())
	m.HandleFunc("GET /api/bots", func(w http.ResponseWriter, r *http.Request) {
		bots, err := s.Catalog.Registry.List()
		if err != nil {
			problem(w, err)
			return
		}
		type view struct {
			Bot           config.Bot      `json:"bot"`
			Effective     config.Settings `json:"effective"`
			Compatibility string          `json:"compatibility"`
		}
		out := []view{}
		for _, b := range bots {
			settings, _ := config.Resolve(b.Settings, config.Settings{})
			status := "compatible (prompt-only)"
			if b.BaseModel != s.Catalog.Model {
				status = "base model mismatch"
			}
			out = append(out, view{b, settings, status})
		}
		respond(w, 200, out)
	})
	m.HandleFunc("POST /api/packages", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Directory string `json:"directory"`
		}
		if err := body(w, r, &req); err != nil {
			problem(w, err)
			return
		}
		b, err := s.Catalog.Install(req.Directory)
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 201, b)
	})
	m.HandleFunc("GET /api/jobs", func(w http.ResponseWriter, r *http.Request) {
		jobs := s.Queue.List()
		sort.Slice(jobs, func(i, j int) bool { return jobs[i].Created.After(jobs[j].Created) })
		respond(w, 200, jobs)
	})
	m.HandleFunc("POST /api/jobs", func(w http.ResponseWriter, r *http.Request) {
		var req Request
		if err := body(w, r, &req); err != nil {
			problem(w, err)
			return
		}
		j, err := s.Queue.Submit(req)
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 202, j)
	})
	m.HandleFunc("GET /api/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		j, ok := s.Queue.Get(r.PathValue("id"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		respond(w, 200, j)
	})
	m.HandleFunc("POST /api/jobs/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Queue.Cancel(r.PathValue("id")); err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, map[string]bool{"cancel_requested": true})
	})
	m.HandleFunc("GET /api/jobs/{id}/events", s.events)
	m.HandleFunc("GET /api/runs", func(w http.ResponseWriter, r *http.Request) {
		runs, err := s.Store.Runs(100)
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, runs)
	})
	m.HandleFunc("GET /api/runs/{id}/trace", func(w http.ResponseWriter, r *http.Request) {
		events, err := s.Store.Trace(r.PathValue("id"))
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, events)
	})
	return m
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.Queue.Get(id); !ok {
		http.NotFound(w, r)
		return
	}
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unavailable", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	last := ""
	for {
		j, _ := s.Queue.Get(id)
		data, _ := json.Marshal(j)
		if string(data) != last {
			last = string(data)
			if _, err := io.WriteString(w, "event: job\ndata: "+last+"\n\n"); err != nil {
				return
			}
			f.Flush()
		}
		if j.Status != "queued" && j.Status != "running" {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}
