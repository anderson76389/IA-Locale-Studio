package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBenchmarkLoopback(t *testing.T) {
	for _, s := range []string{"http://192.168.1.2:11434", "https://example.org", "http://localhost/path"} {
		if _, e := benchmarkBase(s); e == nil {
			t.Fatal(s)
		}
	}
	if _, e := benchmarkBase("http://[::1]:11434"); e != nil {
		t.Fatal(e)
	}
}
func TestSelectModel(t *testing.T) {
	var m modelList
	json.Unmarshal([]byte(`{"models":[{"name":"cloud","size":1,"details":{"format":"gguf"}},{"name":"huge","size":999999999999,"details":{"format":"gguf"}},{"name":"local","size":100,"details":{"format":"gguf"}},{"name":"remote","size":1,"remote_host":"x","details":{"format":"gguf"}}]}`), &m)
	v, e := selectModel(m.Models, "auto")
	if e != nil || v.Name != "local" {
		t.Fatal(v, e)
	}
	if _, e = selectModel(m.Models, "remote"); e == nil {
		t.Fatal("remote accepted")
	}
}
func TestBenchmarkFlow(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			w.Write([]byte(`{"models":[{"name":"local","size":100,"details":{"format":"gguf"}}]}`))
		case "/api/ps":
			if calls == 1 {
				w.Write([]byte(`{"models":[{"name":"local","size_vram":100}]}`))
			} else {
				w.Write([]byte(`{"models":[]}`))
			}
		case "/api/generate":
			var p map[string]any
			json.NewDecoder(r.Body).Decode(&p)
			calls++
			if p["model"] != "local" || p["stream"] != false {
				t.Error(p)
			}
			if calls == 2 && p["keep_alive"] != float64(0) {
				t.Error(p)
			}
			w.Write([]byte(`{"response":"42","done":true,"eval_count":2,"eval_duration":1000000000}`))
		default:
			t.Error(r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer s.Close()
	report := Report{}
	runBenchmark(&report, s.URL, "auto", time.Second)
	if calls != 2 {
		t.Fatal(calls)
	}
	for _, name := range []string{"Génération Texte", "Génération Code", "VRAM modèle chargé", "Libération du modèle"} {
		ok := false
		for _, c := range report.Checks {
			if c.Name == name && c.State == "OK" {
				ok = true
			}
		}
		if !ok {
			t.Fatal(name, report)
		}
	}
}
func TestBenchmarkBusy(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "generate") {
			t.Error("generation while busy")
		}
		w.Write([]byte(`{"models":[{"name":"local","size":100,"details":{"format":"gguf"}}]}`))
	}))
	defer s.Close()
	report := Report{}
	runBenchmark(&report, s.URL, "auto", time.Second)
	if len(report.Checks) != 1 || report.Checks[0].State != "NON_TESTE" {
		t.Fatal(report)
	}
}
