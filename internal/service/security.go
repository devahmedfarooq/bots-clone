package service

import (
	"net/http"
	"strings"
)

// Secure confines the loopback service: exact Host, same-origin browsers only, strict POST content types.
func Secure(next http.Handler, host string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if r.Host != host {
			http.Error(w, "invalid host", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+host {
			http.Error(w, "origin denied", http.StatusForbidden)
			return
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site == "cross-site" {
			http.Error(w, "cross-site request denied", http.StatusForbidden)
			return
		}
		isAPI := strings.HasPrefix(r.URL.Path, "/api/")
		if isAPI || strings.HasPrefix(r.URL.Path, "/ui/") {
			contentType := "application/x-www-form-urlencoded"
			if isAPI {
				contentType = "application/json"
			}
			if r.Method == http.MethodPost && strings.Split(r.Header.Get("Content-Type"), ";")[0] != contentType {
				http.Error(w, "invalid content type", http.StatusUnsupportedMediaType)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
