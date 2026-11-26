package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalURL(t *testing.T) {
	for _, s := range []string{"http://localhost:1420/", "https://192.168.1.2:1420/", "http://[::1]:1420/"} {
		if _, e := localURL(s); e != nil {
			t.Error(s, e)
		}
	}
	for _, s := range []string{"https://example.com", "http://user:password@localhost", "http://localhost/?token=abc", "file:///etc/passwd", "http://8.8.8.8"} {
		if _, e := localURL(s); e == nil {
			t.Error("accepted", s)
		}
	}
}
func TestProbe(t *testing.T) {
	for _, tc := range []struct {
		code        int
		body, state string
	}{{200, `{"models":[]}`, "OK"}, {200, `{}`, "NON_VERIFIE"}, {401, `{}`, "PROTEGE"}, {500, `{}`, "NON_VERIFIE"}, {302, `{}`, "NON_VERIFIE"}} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.code); w.Write([]byte(tc.body)) }))
		state, _ := probe(client(), s.URL, "models")
		s.Close()
		if state != tc.state {
			t.Error(state, tc.state)
		}
	}
}
func TestTLSReject(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html>")) }))
	defer s.Close()
	state, _ := probe(client(), s.URL, "html")
	if state != "NON_VERIFIE" {
		t.Error(state)
	}
}
