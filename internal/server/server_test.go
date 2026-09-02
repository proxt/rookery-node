package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/proxt/rookery-node/internal/panelclient"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	return &Server{panel: panelclient.New("http://panel.invalid", "node1", "node-api-key")}
}

func TestHandleSessionRejectsInvalidToken(t *testing.T) {
	s := newTestServer(t)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /session", s.handleSession)

	body := []byte(`{"sdp":"v=0...","token":"not-a-valid-token"}`)
	r := httptest.NewRequest(http.MethodPost, "/session", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleSessionRejectsMissingToken(t *testing.T) {
	s := newTestServer(t)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /session", s.handleSession)

	body := []byte(`{"sdp":"v=0..."}`)
	r := httptest.NewRequest(http.MethodPost, "/session", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleSessionRejectsMalformedBody(t *testing.T) {
	s := newTestServer(t)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /session", s.handleSession)

	r := httptest.NewRequest(http.MethodPost, "/session", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}
