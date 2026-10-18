package service

import "net/http"

func (s *Server) autonomyRoutes(m *http.ServeMux) {
	for path, file := range map[string]string{"/board": "board.html", "/board.js": "board.js", "/board.css": "board.css"} {
		m.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) {
			data, err := assets.ReadFile("web/" + file)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			content := map[string]string{"board.html": "text/html; charset=utf-8", "board.js": "text/javascript; charset=utf-8", "board.css": "text/css; charset=utf-8"}
			w.Header().Set("Content-Type", content[file])
			_, _ = w.Write(data)
		})
	}
	if s.Autonomy == nil {
		return
	}
	m.HandleFunc("GET /api/board", func(w http.ResponseWriter, r *http.Request) {
		b, err := s.Autonomy.Snapshot()
		respond(w, 200, map[string]any{"board": b, "error": err})
	})
	m.HandleFunc("POST /api/goals", func(w http.ResponseWriter, r *http.Request) {
		var req GoalRequest
		if err := body(w, r, &req); err != nil {
			problem(w, err)
			return
		}
		g, err := s.Autonomy.Create(req)
		if err != nil {
			problem(w, err)
			return
		}
		respond(w, 201, g)
	})
	m.HandleFunc("POST /api/goals/{id}/settings", func(w http.ResponseWriter, r *http.Request) {
		var req GoalSettings
		if err := body(w, r, &req); err != nil {
			problem(w, err)
			return
		}
		if err := s.Autonomy.Settings(r.PathValue("id"), req); err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, map[string]bool{"saved": true})
	})
	m.HandleFunc("POST /api/goals/{id}/chat", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Text string `json:"text"`
		}
		if err := body(w, r, &req); err != nil {
			problem(w, err)
			return
		}
		if err := s.Autonomy.Chat(r.PathValue("id"), req.Text); err != nil {
			problem(w, err)
			return
		}
		respond(w, 201, map[string]bool{"saved": true})
	})
	m.HandleFunc("POST /api/cards/{id}/{action}", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Note string `json:"note"`
		}
		if err := body(w, r, &req); err != nil {
			problem(w, err)
			return
		}
		if err := s.Autonomy.Action(r.PathValue("id"), r.PathValue("action"), req.Note); err != nil {
			problem(w, err)
			return
		}
		respond(w, 200, map[string]bool{"saved": true})
	})
}
