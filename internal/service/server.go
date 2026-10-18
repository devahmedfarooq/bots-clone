package service

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"bots/internal/application"
	"bots/internal/config"
	"bots/internal/state"
	"bots/internal/workflow"
)

type Server struct {
	Catalog     application.Catalog
	Queue       *Queue
	Store       *state.Store
	Workflows   *Workflows
	Autonomy    *Autonomy
	PackageHome string
}

func respond(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	data, err := state.Sanitized(v)
	if err != nil {
		return
	}
	_, _ = w.Write(data)
}

func problem(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, ErrQueueFull) {
		status = http.StatusTooManyRequests
	}
	respond(w, status, map[string]string{"error": state.Redact(err.Error())})
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
	s.dashboardRoutes(m)
	s.autonomyRoutes(m)
	m.HandleFunc("POST /api/route", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Input string `json:"input"`
			Bot   string `json:"bot,omitempty"`
		}
		if err := body(w, r, &req); err != nil {
			problem(w, err)
			return
		}
		route, err := s.Catalog.Route(req.Input, req.Bot)
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, route)
	})
	if s.Workflows != nil {
		m.HandleFunc("POST /api/workflows/{id}/resume", func(w http.ResponseWriter, r *http.Request) {
			f, err := s.Workflows.Resume(r.PathValue("id"))
			if err != nil {
				problem(w, err)
				return
			}
			respond(w, 202, f)
		})
		m.HandleFunc("GET /api/workflows", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, s.Workflows.List()) })
		m.HandleFunc("POST /api/workflows", func(w http.ResponseWriter, r *http.Request) {
			var d workflow.Definition
			if err := body(w, r, &d); err != nil {
				problem(w, err)
				return
			}
			f, err := s.Workflows.Start(d)
			if err != nil {
				problem(w, err)
				return
			}
			respond(w, 202, f)
		})
		m.HandleFunc("GET /api/workflows/{id}", func(w http.ResponseWriter, r *http.Request) {
			f, ok := s.Workflows.Get(r.PathValue("id"))
			if !ok {
				http.NotFound(w, r)
				return
			}
			respond(w, 200, f)
		})
		m.HandleFunc("POST /api/workflows/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
			if err := s.Workflows.Cancel(r.PathValue("id")); err != nil {
				problem(w, err)
				return
			}
			respond(w, 200, map[string]bool{"cancel_requested": true})
		})
	}
	m.HandleFunc("GET /api/bots", func(w http.ResponseWriter, r *http.Request) {
		out, err := s.botViews()
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, out)
	})
	m.HandleFunc("POST /api/packages", func(w http.ResponseWriter, r *http.Request) {
		var req packageRequest
		if err := body(w, r, &req); err != nil {
			problem(w, err)
			return
		}
		b, err := s.installPackage(req)
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 201, b)
	})
	m.HandleFunc("GET /api/jobs", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, s.sortedJobs())
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
