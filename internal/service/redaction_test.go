package service

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPRedaction(t *testing.T) {
	w := httptest.NewRecorder()
	respond(w, 200, map[string]any{"password": "fixture-secret", "output": "api_key=fixture-key Bearer fixture-token"})
	for _, secret := range []string{"fixture-secret", "fixture-key", "fixture-token"} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatal("secret leaked", secret)
		}
	}
}
