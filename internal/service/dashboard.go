package service

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"bots/internal/config"
	"bots/internal/state"
)

//go:embed web/*
var assets embed.FS

//go:embed templates/*.html
var views embed.FS

var dashboardTemplates = template.Must(template.New("dashboard").Funcs(template.FuncMap{
	"dict": func(values ...any) (map[string]any, error) {
		if len(values)%2 != 0 {
			return nil, errors.New("component properties require key/value pairs")
		}
		out := map[string]any{}
		for i := 0; i < len(values); i += 2 {
			key, ok := values[i].(string)
			if !ok {
				return nil, errors.New("component property name must be string")
			}
			out[key] = values[i+1]
		}
		return out, nil
	},
	"join":     func(values []string) string { return state.Redact(strings.Join(values, ", ")) },
	"redact":   state.Redact,
	"terminal": func(status string) bool { return status != "queued" && status != "running" },
	"runView": func(prefix string, run any) any {
		return struct {
			Prefix string
			Run    any
		}{prefix, run}
	},
}).ParseFS(views, "templates/*.html"))

type BotView struct {
	Bot           config.Bot      `json:"bot"`
	Effective     config.Settings `json:"effective"`
	Compatibility string          `json:"compatibility"`
}
type workspaceView struct {
	Bots     []BotView
	Selected *BotView
}
type packageRequest struct {
	Directory string `json:"directory"`
	Archive   string `json:"archive,omitempty"`
}

func (s *Server) botViews() ([]BotView, error) {
	bots, err := s.Catalog.Registry.List()
	if err != nil {
		return nil, err
	}
	out := []BotView{}
	for _, b := range bots {
		settings, err := config.Resolve(b.Settings, config.Settings{})
		if err != nil {
			return nil, err
		}
		compatibility := "compatible (prompt-only)"
		if b.Adapter != "" {
			compatibility = "compatible (experimental adapter)"
		}
		if err := s.Catalog.Check(b); err != nil {
			compatibility = err.Error()
		}
		if b.BaseModel != s.Catalog.Model {
			compatibility = "base model mismatch"
		}
		out = append(out, BotView{b, settings, compatibility})
	}
	return out, nil
}

func (s *Server) sortedJobs() []Job {
	jobs := s.Queue.List()
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].Created.After(jobs[j].Created) })
	return jobs
}

func (s *Server) installPackage(req packageRequest) (config.Bot, error) {
	if (req.Directory == "") == (req.Archive == "") {
		return config.Bot{}, errors.New("specify exactly one directory or archive")
	}
	if req.Archive != "" {
		home := s.PackageHome
		if home == "" {
			home = filepath.Join(filepath.Dir(s.Catalog.Registry.Dir), "packages")
		}
		return s.Catalog.Import(req.Archive, home)
	}
	return s.Catalog.Install(req.Directory)
}

func render(w http.ResponseWriter, name string, data any) {
	var output bytes.Buffer
	if err := dashboardTemplates.ExecuteTemplate(&output, name, data); err != nil {
		http.Error(w, "Dashboard rendering failed.", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(output.Bytes())
}
func uiProblem(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, ErrQueueFull) {
		status = 429
	}
	http.Error(w, state.Redact(err.Error()), status)
}
func uiForm(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, (1<<20)+4096)
	return r.ParseForm()
}

func (s *Server) dashboardRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) { render(w, "page", nil) })
	files, _ := fs.Sub(assets, "web")
	static := http.FileServer(http.FS(files))
	for _, path := range []string{"/style.css", "/app.js", "/htmx.min.js"} {
		m.Handle("GET "+path, static)
	}
	for path, name := range map[string]string{"/ui/workspace": "workspace", "/ui/bots": "bots", "/ui/bots/picker": "picker-update"} {
		m.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) {
			bots, err := s.botViews()
			if err != nil {
				uiProblem(w, err)
				return
			}
			view := workspaceView{Bots: bots}
			if len(bots) > 0 {
				view.Selected = &bots[0]
			}
			render(w, name, view)
		})
	}
	m.HandleFunc("GET /ui/bots/settings", func(w http.ResponseWriter, r *http.Request) {
		bots, err := s.botViews()
		if err != nil {
			uiProblem(w, err)
			return
		}
		for _, bot := range bots {
			if bot.Bot.ID == r.URL.Query().Get("bot") {
				render(w, "settings", bot)
				return
			}
		}
		uiProblem(w, errors.New("bot not found"))
	})
	m.HandleFunc("GET /ui/jobs", func(w http.ResponseWriter, r *http.Request) { render(w, "jobs", s.sortedJobs()) })
	m.HandleFunc("GET /ui/runs", func(w http.ResponseWriter, r *http.Request) {
		runs, err := s.Store.Runs(20)
		if err != nil {
			uiProblem(w, err)
			return
		}
		render(w, "runs", runs)
	})
	m.HandleFunc("GET /ui/runs/{id}/trace", func(w http.ResponseWriter, r *http.Request) {
		events, err := s.Store.Trace(r.PathValue("id"))
		if err != nil {
			uiProblem(w, err)
			return
		}
		data, err := json.MarshalIndent(events, "", "  ")
		if err != nil {
			uiProblem(w, err)
			return
		}
		render(w, "trace", state.Redact(string(data)))
	})
	m.HandleFunc("POST /ui/jobs", func(w http.ResponseWriter, r *http.Request) {
		if err := uiForm(w, r); err != nil {
			uiProblem(w, err)
			return
		}
		if _, err := s.Queue.Submit(Request{Bot: r.PostForm.Get("bot"), Input: r.PostForm.Get("input")}); err != nil {
			uiProblem(w, err)
			return
		}
		w.Header().Set("HX-Trigger", "jobsChanged")
		render(w, "notice", "Task submitted.")
	})
	m.HandleFunc("POST /ui/packages", func(w http.ResponseWriter, r *http.Request) {
		if err := uiForm(w, r); err != nil {
			uiProblem(w, err)
			return
		}
		req := packageRequest{}
		switch r.PostForm.Get("kind") {
		case "directory":
			req.Directory = r.PostForm.Get("path")
		case "archive":
			req.Archive = r.PostForm.Get("path")
		default:
			uiProblem(w, errors.New("invalid package format"))
			return
		}
		if _, err := s.installPackage(req); err != nil {
			uiProblem(w, err)
			return
		}
		w.Header().Set("HX-Trigger", "botsChanged")
		render(w, "notice", "Bot installed. Review its tools and folders.")
	})
	m.HandleFunc("POST /ui/jobs/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Queue.Cancel(r.PathValue("id")); err != nil {
			uiProblem(w, err)
			return
		}
		w.Header().Set("HX-Trigger", "jobsChanged")
		render(w, "notice", "Cancellation requested.")
	})
}
