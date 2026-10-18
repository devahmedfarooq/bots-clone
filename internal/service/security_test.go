package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityBoundaries(t *testing.T) {
	h := Secure(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }), "127.0.0.1:8080", "fixture-token")
	for _, test := range []struct {
		name, host, origin, auth, method, content string
		code                                      int
	}{
		{"authorized", "127.0.0.1:8080", "http://127.0.0.1:8080", "Bearer fixture-token", "POST", "application/json", 204},
		{"no auth", "127.0.0.1:8080", "", "", "GET", "", 401},
		{"foreign origin", "127.0.0.1:8080", "http://evil.test", "Bearer fixture-token", "POST", "application/json", 403},
		{"rebound host", "evil.test", "", "Bearer fixture-token", "GET", "", 403},
		{"form post", "127.0.0.1:8080", "", "Bearer fixture-token", "POST", "text/plain", 415},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := httptest.NewRequest(test.method, "http://"+test.host+"/api/jobs", nil)
			r.Header.Set("Origin", test.origin)
			r.Header.Set("Authorization", test.auth)
			r.Header.Set("Content-Type", test.content)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != test.code {
				t.Fatal(w.Code, w.Body.String())
			}
		})
	}
}
