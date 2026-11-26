package main

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func netResponse(r *http.Request, code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}
}
func TestOfflineLocalGeneration(t *testing.T) {
	s := studioWith(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			io.WriteString(w, `{"models":[{"name":"local:1","size":1234,"details":{"format":"gguf"}}]}`)
		case "/api/chat":
			io.WriteString(w, `{"message":{"content":"local"},"done":true}`+"\n")
		default:
			http.NotFound(w, r)
		}
	})
	base := s.c.Transport
	var outbound atomic.Int32
	s.c.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Scheme == "https" {
			outbound.Add(1)
			return nil, errors.New("offline")
		}
		return base.RoundTrip(r)
	})
	if w := req(t, s, "GET", "/api/status", nil, true); w.Code != 200 {
		t.Fatal(w.Code)
	}
	launch(t, s, Request{Kind: "texte", Prompt: "test", Model: "local:1", Context: 2048, Tokens: 128})
	if j := await(t, s); j.State != "done" {
		t.Fatal(j)
	}
	if outbound.Load() != 0 {
		t.Fatal("unexpected Internet request")
	}
	for _, path := range []string{"/api/library?kind=texte", "/api/connectivity"} {
		w := req(t, s, "GET", path, nil, true)
		if w.Code != 503 || !strings.Contains(w.Body.String(), "Internet") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	w := req(t, s, "POST", "/api/pull", map[string]string{"model": "qwen2.5:7b"}, true)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	if j := await(t, s); j.State != "error" {
		t.Fatal(j)
	}
}
func TestPullNetworkFailure(t *testing.T) {
	s := studioWith(t, func(w http.ResponseWriter, r *http.Request) { t.Error("engine should not be called offline") })
	s.c.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) { return nil, errors.New("offline") })
	w := req(t, s, "POST", "/api/pull", map[string]string{"model": "qwen:7b"}, true)
	if w.Code != 202 {
		t.Fatal(w.Code)
	}
	j := await(t, s)
	if j.State != "error" || !strings.Contains(j.Error, "Internet") {
		t.Fatal(j)
	}
}
func TestPullStream(t *testing.T) {
	for _, tc := range []struct {
		name, stream, want string
		installed          bool
	}{{"ok", `{"status":"pulling","total":100,"completed":50}` + "\n" + `{"status":"success"}`, "done", true}, {"truncated", `{"status":"pulling","total":100,"completed":50}`, "error", true}, {"missing", `{"status":"success"}`, "error", false}, {"remote", `{"error":"connection reset"}`, "error", true}} {
		t.Run(tc.name, func(t *testing.T) {
			s := studioWith(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/pull":
					io.WriteString(w, tc.stream+"\n")
				case "/api/tags":
					if tc.installed {
						io.WriteString(w, `{"models":[{"name":"qwen2.5:7b","size":1234,"details":{"format":"gguf"}}]}`)
					} else {
						io.WriteString(w, `{"models":[]}`)
					}
				default:
					http.NotFound(w, r)
				}
			})
			base := s.c.Transport
			s.c.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host == "registry.ollama.ai" {
					return netResponse(r, 401, ""), nil
				}
				return base.RoundTrip(r)
			})
			w := req(t, s, "POST", "/api/pull", map[string]string{"model": "qwen2.5:7b"}, true)
			if w.Code != 202 {
				t.Fatal(w.Code, w.Body.String())
			}
			j := await(t, s)
			if j.State != tc.want {
				t.Fatal(j)
			}
		})
	}
}
func TestPullNameRestrictions(t *testing.T) {
	for _, name := range []string{"qwen2.5:7b", "llama3.2", "namespace/model:latest"} {
		if !pullModelID(name) {
			t.Fatal(name)
		}
	}
	for _, name := range []string{"https://evil/model", "../x", "qwen:cloud", "host.example:123/model", "x y"} {
		if pullModelID(name) {
			t.Fatal(name)
		}
	}
}
