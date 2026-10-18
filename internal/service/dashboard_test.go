package service

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"bots/internal/state"
)

func TestDashboardEscapesAndRedactsModelOutput(t *testing.T) {
	w := httptest.NewRecorder()
	r := state.Run{ID: "fixture", Status: "completed", Output: `<script>alert('x')</script> api_key=fixture-secret`}
	render(w, "run-output", struct {
		Prefix string
		Run    state.Run
	}{"test", r})
	if w.Code != 200 || strings.Contains(w.Body.String(), "<script>") || strings.Contains(w.Body.String(), "fixture-secret") || !strings.Contains(w.Body.String(), "hx-preserve") {
		t.Fatal(w.Body.String())
	}
}

func TestUIUsesProtectedFragmentsAndBoundedForms(t *testing.T) {
	q, _, store := testQueue(t, 4)
	s := &Server{Catalog: q.catalog, Queue: q, Store: store}
	h := Secure(s.Handler(), "127.0.0.1:8080", "fixture-token")
	for _, test := range []struct {
		method, path, auth, content, data string
		status                            int
	}{
		{"GET", "/ui/workspace", "", "", "", 401},
		{"GET", "/ui/workspace", "Bearer fixture-token", "", "", 200},
		{"POST", "/ui/jobs", "Bearer fixture-token", "application/json", "{}", 415},
		{"POST", "/ui/jobs", "Bearer fixture-token", "application/x-www-form-urlencoded", "bot=missing&input=hello", 400},
		{"POST", "/ui/jobs", "Bearer fixture-token", "application/x-www-form-urlencoded", strings.Repeat("x", (1<<20)+4097), 400},
	} {
		r := httptest.NewRequest(test.method, "http://127.0.0.1:8080"+test.path, strings.NewReader(test.data))
		r.Header.Set("Authorization", test.auth)
		r.Header.Set("Content-Type", test.content)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != test.status {
			t.Fatalf("%s %s: %d %s", test.method, test.path, w.Code, w.Body.String())
		}
	}
	if len(q.List()) != 0 {
		t.Fatal("invalid UI request scheduled a job")
	}
	r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || strings.Contains(w.Body.String(), "fixture-token") || !strings.Contains(w.Body.String(), "/htmx.min.js") {
		t.Fatal(w.Body.String())
	}
}

func TestFirstBotInstallEnablesTaskSubmission(t *testing.T) {
	q, _, store := testQueue(t, 4)
	if err := q.catalog.Registry.Remove("echo"); err != nil {
		t.Fatal(err)
	}
	s := &Server{Catalog: q.catalog, Queue: q, Store: store}
	h := Secure(s.Handler(), "127.0.0.1:8080", "fixture-token")
	request := func(method, path, data string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://127.0.0.1:8080"+path, strings.NewReader(data))
		r.Header.Set("Authorization", "Bearer fixture-token")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	empty := request("GET", "/ui/workspace", "")
	if !strings.Contains(empty.Body.String(), "disabled") {
		t.Fatal("empty dashboard allows submission")
	}
	form := url.Values{"kind": {"directory"}, "path": {filepath.Dir(q.catalog.Registry.Dir)}}
	installed := request("POST", "/ui/packages", form.Encode())
	if installed.Code != 200 || installed.Header().Get("HX-Trigger") != "botsChanged" {
		t.Fatal(installed.Code, installed.Body.String())
	}
	picker := request("GET", "/ui/bots/picker", "")
	if picker.Code != 200 || strings.Contains(picker.Body.String(), "disabled") || !strings.Contains(picker.Body.String(), "innerHTML:#submit-area") {
		t.Fatal(picker.Body.String())
	}
}
