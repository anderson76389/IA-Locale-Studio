package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func req(t *testing.T, s *Studio, method, path string, body any, auth bool) *httptest.ResponseRecorder {
	t.Helper()
	var b bytes.Buffer
	if body != nil {
		if e := json.NewEncoder(&b).Encode(body); e != nil {
			t.Fatal(e)
		}
	}
	r := httptest.NewRequest(method, "http://localhost:1420"+path, &b)
	if auth {
		r.Header.Set("X-Studio-Token", s.token)
	}
	w := httptest.NewRecorder()
	s.serve(w, r)
	return w
}
func studioWith(t *testing.T, handler http.HandlerFunc) *Studio {
	t.Helper()
	engine := httptest.NewServer(handler)
	t.Cleanup(engine.Close)
	s, e := newStudio(t.TempDir(), engine.URL, engine.URL)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func await(t *testing.T, s *Studio) Job {
	t.Helper()
	end := time.Now().Add(8 * time.Second)
	for time.Now().Before(end) {
		j := s.snapshot()
		if j.State != "running" && j.State != "stopping" {
			return j
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("job did not finish")
	return Job{}
}
func launch(t *testing.T, s *Studio, q Request) {
	t.Helper()
	w := req(t, s, "POST", "/api/generate", q, true)
	if w.Code != 202 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}
func TestStudioSecurity(t *testing.T) {
	s := studioWith(t, func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected external request") })
	for _, x := range []struct {
		method, path string
		auth         bool
		want         int
	}{{"GET", "/", false, 200}, {"GET", "/api/session", false, 200}, {"GET", "/api/job", false, 403}, {"POST", "/api/backup", false, 403}, {"GET", "/api/file?job=../&name=x.png", true, 400}, {"POST", "/api/generate", true, 400}, {"GET", "/api/unknown", true, 404}} {
		w := req(t, s, x.method, x.path, nil, x.auth)
		if w.Code != x.want {
			t.Errorf("%s %d want %d", x.path, w.Code, x.want)
		}
	}
	for _, host := range []string{"evil.example:1420", "localhost:1420"} {
		r := httptest.NewRequest("GET", "http://"+host+"/api/session", nil)
		r.Header.Set("Origin", "https://evil.example")
		w := httptest.NewRecorder()
		s.serve(w, r)
		if w.Code != 403 {
			t.Fatal(w.Code)
		}
	}
}
func TestStudioTextCode(t *testing.T) {
	for _, k := range []string{"texte", "code"} {
		t.Run(k, func(t *testing.T) {
			s := studioWith(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/tags":
					io.WriteString(w, `{"models":[{"name":"local:1","size":1234,"details":{"format":"gguf"}}]}`)
				case "/api/chat":
					var d map[string]any
					json.NewDecoder(r.Body).Decode(&d)
					if d["model"] != "local:1" || d["keep_alive"] != float64(0) {
						t.Errorf("wrong payload %v", d)
					}
					io.WriteString(w, "{\"message\":{\"content\":\"bonjour\"},\"done\":false}\n{\"message\":{\"content\":\" !\"},\"done\":true,\"eval_count\":3,\"eval_duration\":1000000000}\n")
				default:
					http.NotFound(w, r)
				}
			})
			launch(t, s, Request{Kind: k, Prompt: "Test", Model: "local:1", Context: 2048, Tokens: 128})
			j := await(t, s)
			if j.State != "done" || j.Text != "bonjour !" || j.Rate != 3 || len(j.Files) != 2 {
				t.Fatalf("%+v", j)
			}
			b, e := os.ReadFile(filepath.Join(s.outputDir(j.ID), "conversation.json"))
			if e != nil || !strings.Contains(string(b), "Test") {
				t.Fatal("conversation not saved", e)
			}
			r := req(t, s, "GET", "/api/file?job="+j.ID+"&name=reponse.txt", nil, true)
			if r.Code != 200 || r.Body.String() != "bonjour !" {
				t.Fatal(r.Code, r.Body.String())
			}
		})
	}
}
func TestStudioRejectRemoteAndTruncated(t *testing.T) {
	for _, remote := range []bool{true, false} {
		t.Run(fmt.Sprint(remote), func(t *testing.T) {
			s := studioWith(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/tags" {
					if remote {
						io.WriteString(w, `{"models":[{"name":"x:1","size":123,"remote_host":"https://remote","details":{"format":"gguf"}}]}`)
					} else {
						io.WriteString(w, `{"models":[{"name":"x:1","size":123,"details":{"format":"gguf"}}]}`)
					}
					return
				}
				if remote {
					t.Error("remote chat called")
				}
				io.WriteString(w, "{\"message\":{\"content\":\"partial\"},\"done\":false}\n")
			})
			launch(t, s, Request{Kind: "texte", Prompt: "x", Model: "x:1", Context: 2048, Tokens: 128})
			j := await(t, s)
			if j.State != "error" || len(j.Files) != 0 {
				t.Fatalf("%+v", j)
			}
		})
	}
}

const testWF = `{"1":{"class_type":"CLIPTextEncode","inputs":{"text":"{{PROMPT}}"}}}`

func TestWorkflow(t *testing.T) {
	for _, s := range []string{`{}`, `{"nodes":[]}`, `{"1":{"class_type":"Foo","inputs":{"text":"not placeholder"}}}`} {
		if _, e := workflow([]byte(s), "x"); e == nil {
			t.Fatal("invalid workflow accepted")
		}
	}
	w, e := workflow([]byte(testWF), "\"foo\"\nbar")
	if e != nil || w["1"].(map[string]any)["inputs"].(map[string]any)["text"] != "\"foo\"\nbar" {
		t.Fatal(w, e)
	}
}
func TestStudioMedia(t *testing.T) {
	for _, x := range []struct {
		kind, name string
		success    bool
	}{{"image", "a.png", true}, {"video", "a.mp4", true}, {"musique", "a.wav", true}, {"video", "preview.png", false}} {
		t.Run(x.kind+x.name, func(t *testing.T) {
			s := studioWith(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/prompt":
					var d map[string]any
					json.NewDecoder(r.Body).Decode(&d)
					if d["prompt"].(map[string]any)["1"].(map[string]any)["inputs"].(map[string]any)["text"] != "test" {
						t.Error("prompt not inserted")
					}
					io.WriteString(w, `{"prompt_id":"test-prompt"}`)
				case "/history/test-prompt":
					fmt.Fprintf(w, `{"test-prompt":{"status":{"completed":true,"status_str":"success"},"outputs":{"1":{"files":[{"filename":%q,"subfolder":"","type":"output"}]}}}}`, x.name)
				case "/view":
					if r.URL.Query().Get("filename") != x.name {
						t.Error("wrong filename")
					}
					io.WriteString(w, "simulated media bytes")
				default:
					http.NotFound(w, r)
				}
			})
			launch(t, s, Request{Kind: x.kind, Prompt: "test", Workflow: []byte(testWF), Trusted: true})
			j := await(t, s)
			if (j.State == "done") != x.success {
				t.Fatalf("%+v", j)
			}
			if x.success {
				if len(j.Files) != 1 {
					t.Fatal(j.Files)
				}
				b, e := os.ReadFile(filepath.Join(s.outputDir(j.ID), j.Files[0]))
				if e != nil || string(b) != "simulated media bytes" {
					t.Fatal(e)
				}
			}
		})
	}
}
func TestStudioBackupRestore(t *testing.T) {
	s := studioWith(t, func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected network") })
	if e := os.WriteFile(filepath.Join(s.root, "data", "models", "model.bin"), []byte("data"), 0600); e != nil {
		t.Fatal(e)
	}
	w := req(t, s, "POST", "/api/backup", map[string]any{}, true)
	if w.Code != 202 {
		t.Fatal(w.Code)
	}
	j := await(t, s)
	if j.State != "done" {
		t.Fatal(j)
	}
	name := j.ID + ".zip"
	w = req(t, s, "GET", "/api/backups", nil, true)
	if !strings.Contains(w.Body.String(), name) {
		t.Fatal(w.Body.String())
	}
	w = req(t, s, "POST", "/api/restore", map[string]string{"name": name}, true)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	r := await(t, s)
	if r.State != "done" {
		t.Fatal(r)
	}
	b, e := os.ReadFile(filepath.Join(s.root, "restauration-"+r.ID, "IA-Locale-Studio", "models", "model.bin"))
	if e != nil || string(b) != "data" {
		t.Fatal(e, string(b))
	}
}
func TestStudioConcurrentStop(t *testing.T) {
	started := make(chan struct{})
	s := studioWith(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			io.WriteString(w, `{"models":[{"name":"x:1","size":123,"details":{"format":"gguf"}}]}`)
			return
		}
		if r.URL.Path == "/api/chat" {
			io.Copy(io.Discard, r.Body)
			close(started)
			select {
			case <-r.Context().Done():
			case <-time.After(3 * time.Second):
			}
			return
		}
		http.NotFound(w, r)
	})
	launch(t, s, Request{Kind: "texte", Prompt: "x", Model: "x:1", Context: 2048, Tokens: 128})
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("not started")
	}
	if w := req(t, s, "POST", "/api/backup", map[string]any{}, true); w.Code != 409 {
		t.Fatal(w.Code)
	}
	if w := req(t, s, "POST", "/api/stop", map[string]any{}, true); w.Code != 200 {
		t.Fatal(w.Code)
	}
	j := await(t, s)
	if j.State != "stopped" {
		t.Fatal(j)
	}
}

func TestHistoryRestartResumeDelete(t *testing.T) {
	var calls int
	engine := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			io.WriteString(w, `{"models":[{"name":"x:1","size":123,"details":{"format":"gguf"}}]}`)
			return
		}
		var body struct{ Messages []Message }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		calls++
		if calls == 2 && (len(body.Messages) != 3 || body.Messages[0].Content != "Premier") {
			t.Error("contexte perdu", body.Messages)
		}
		io.WriteString(w, `{"message":{"content":"OK"},"done":true}`)
	}
	s := studioWith(t, engine)
	q := Request{Kind: "texte", Model: "x:1", Prompt: "Premier", Context: 2048, Tokens: 64}
	launch(t, s, q)
	first := await(t, s)
	restarted, e := newStudio(s.root, s.ollama, s.comfy)
	if e != nil {
		t.Fatal(e)
	}
	q.Thread = first.ID
	q.Prompt = "Suite"
	launch(t, restarted, q)
	second := await(t, restarted)
	if second.State != "done" {
		t.Fatal(second.Error)
	}
	q.Kind = "code"
	launch(t, restarted, q)
	if await(t, restarted).State != "error" {
		t.Fatal("atelier incorrect accepté")
	}
	q.Thread = ""
	launch(t, restarted, q)
	separate := await(t, restarted)
	deleted := req(t, restarted, "POST", "/api/history/delete", map[string]string{"thread": first.ID}, true)
	if deleted.Code != 200 {
		t.Fatal(deleted.Body.String())
	}
	for _, job := range []Job{first, second} {
		if _, e := os.Stat(s.outputDir(job.ID)); !os.IsNotExist(e) {
			t.Fatal("sortie conservée", job.ID)
		}
	}
	rows, e := restarted.turns()
	if e != nil || len(rows) != 1 || rows[0].ID != separate.ID {
		t.Fatal(rows, e)
	}
}
