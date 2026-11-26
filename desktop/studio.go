package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

//go:embed web/*
var webFiles embed.FS
var validID = regexp.MustCompile(`^[a-f0-9]{32}$`)
var modelID = regexp.MustCompile(`^[A-Za-z0-9_.:/-]{1,180}$`)
var promptID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
var mediaExt = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".gif": true, ".mp4": true, ".webm": true, ".wav": true, ".mp3": true, ".flac": true, ".ogg": true, ".m4a": true}

const maxOutput = 2 << 30

type Job struct {
	ID       string    `json:"id"`
	Kind     string    `json:"kind"`
	State    string    `json:"state"`
	Text     string    `json:"text"`
	Error    string    `json:"error,omitempty"`
	Files    []string  `json:"files"`
	Started  time.Time `json:"started"`
	Progress string    `json:"progress,omitempty"`
	Rate     float64   `json:"rate,omitempty"`
	PromptID string    `json:"prompt_id,omitempty"`
}
type Request struct {
	Kind     string          `json:"kind"`
	Prompt   string          `json:"prompt"`
	Model    string          `json:"model"`
	Context  int             `json:"context"`
	Tokens   int             `json:"tokens"`
	Workflow json.RawMessage `json:"workflow"`
	Trusted  bool            `json:"trusted"`
	History  []Message       `json:"history"`
}
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type Studio struct {
	root, token, ollama, comfy string
	mu                         sync.Mutex
	job                        Job
	cancel                     context.CancelFunc
	c                          *http.Client
	obsMu                      sync.Mutex
	obs                        Report
	obsAt                      time.Time
}

func id() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b[:])
}
func newStudio(root, ollama, comfy string) (*Studio, error) {
	for _, p := range []string{"data", "data/outputs", "data/models", "data/logs", "backups"} {
		if e := os.MkdirAll(filepath.Join(root, p), 0700); e != nil {
			return nil, e
		}
	}
	a, e := benchmarkBase(ollama)
	if e != nil {
		return nil, e
	}
	b, e := benchmarkBase(comfy)
	if e != nil {
		return nil, e
	}
	return &Studio{root: root, token: id() + id(), ollama: a, comfy: b, c: &http.Client{Transport: &http.Transport{DialContext: (&net.Dialer{Timeout: 4 * time.Second}).DialContext, ResponseHeaderTimeout: 45 * time.Second}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func reject(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("JSON supplémentaire interdit")
	}
	return nil
}
func (s *Studio) snapshot() Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	j := s.job
	j.Files = append([]string(nil), j.Files...)
	return j
}
func (s *Studio) change(f func(*Job)) { s.mu.Lock(); defer s.mu.Unlock(); f(&s.job) }
func (s *Studio) active() bool        { return s.job.State == "running" || s.job.State == "stopping" }
func (s *Studio) start(kind string, task func(context.Context, string) error) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active() {
		return Job{}, errors.New("Une opération est déjà en cours")
	}
	j := Job{ID: id(), Kind: kind, State: "running", Started: time.Now(), Files: []string{}}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	s.job = j
	s.cancel = cancel
	go func() {
		defer cancel()
		e := task(ctx, j.ID)
		s.change(func(p *Job) {
			if e != nil {
				p.State = "error"
				p.Error = e.Error()
			} else {
				p.State = "done"
			}
			if ctx.Err() != nil && e != nil {
				p.State = "stopped"
				p.Error = "Attente locale interrompue. L'arrêt du moteur n'est pas garanti : vérifier son état. " + e.Error()
			}
			s.cancel = nil
		})
	}()
	return j, nil
}
func (s *Studio) outputDir(job string) string { return filepath.Join(s.root, "data", "outputs", job) }
func (s *Studio) localModels(ctx context.Context) ([]modelInfo, error) {
	var l modelList
	e := api(ctx, s.c, s.ollama, "/api/tags", nil, &l)
	if e != nil {
		return nil, e
	}
	m := []modelInfo{}
	for _, x := range l.Models {
		n := modelName(x)
		if modelID.MatchString(n) && x.Size > 0 && strings.EqualFold(x.Details.Format, "gguf") && x.RemoteHost == "" && x.RemoteModel == "" && !strings.Contains(strings.ToLower(n), ":cloud") && !strings.HasSuffix(strings.ToLower(n), "-cloud") {
			m = append(m, x)
		}
	}
	return m, nil
}
func (s *Studio) chat(ctx context.Context, job string, q Request) error {
	ms, e := s.localModels(ctx)
	if e != nil {
		return fmt.Errorf("Ollama : %w", e)
	}
	ok := false
	for _, m := range ms {
		if modelName(m) == q.Model {
			ok = true
		}
	}
	if !ok {
		return errors.New("Modèle local non détecté ou modèle distant refusé")
	}
	messages := append([]Message{}, q.History...)
	if q.Kind == "code" {
		messages = append([]Message{{"system", "Tu es un assistant de programmation. Réponds précisément et explique les risques."}}, messages...)
	}
	messages = append(messages, Message{"user", q.Prompt})
	data, _ := json.Marshal(map[string]any{"model": q.Model, "messages": messages, "stream": true, "keep_alive": 0, "options": map[string]int{"num_ctx": q.Context, "num_predict": q.Tokens}})
	req, e := http.NewRequestWithContext(ctx, "POST", s.ollama+"/api/chat", bytes.NewReader(data))
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/json")
	res, e := s.c.Do(req)
	if e != nil {
		return e
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("Ollama HTTP %d", res.StatusCode)
	}
	scan := bufio.NewScanner(res.Body)
	scan.Buffer(make([]byte, 64<<10), 2<<20)
	var b strings.Builder
	done := false
	rate := float64(0)
	for scan.Scan() {
		var x struct {
			Message      Message `json:"message"`
			Done         bool    `json:"done"`
			Error        string  `json:"error"`
			EvalCount    int     `json:"eval_count"`
			EvalDuration int64   `json:"eval_duration"`
		}
		if e = json.Unmarshal(scan.Bytes(), &x); e != nil {
			return e
		}
		if x.Error != "" {
			return errors.New(x.Error)
		}
		if b.Len()+len(x.Message.Content) > 4<<20 {
			return errors.New("Réponse supérieure à 4 Mio")
		}
		b.WriteString(x.Message.Content)
		s.change(func(j *Job) { j.Text += x.Message.Content })
		if x.Done {
			done = true
			if x.EvalDuration > 0 {
				rate = float64(x.EvalCount) * 1e9 / float64(x.EvalDuration)
			}
			break
		}
	}
	if e = scan.Err(); e != nil {
		return e
	}
	if !done {
		return errors.New("Flux interrompu avant confirmation de fin")
	}
	if b.Len() == 0 {
		return errors.New("Le moteur n'a fourni aucun texte")
	}
	if e = os.Mkdir(s.outputDir(job), 0700); e != nil {
		return e
	}
	if e = os.WriteFile(filepath.Join(s.outputDir(job), "reponse.txt"), []byte(b.String()), 0600); e != nil {
		return e
	}
	messages = append(messages, Message{"assistant", b.String()})
	data, e = json.MarshalIndent(map[string]any{"model": q.Model, "kind": q.Kind, "messages": messages}, "", "  ")
	if e != nil {
		return e
	}
	if e = os.WriteFile(filepath.Join(s.outputDir(job), "conversation.json"), data, 0600); e != nil {
		return e
	}
	s.change(func(j *Job) { j.Rate = rate; j.Files = []string{"reponse.txt", "conversation.json"} })
	return nil
}

// Only explicit, exact string placeholders are replaced; no string-to-JSON interpolation.
func workflow(raw json.RawMessage, prompt string) (map[string]any, error) {
	var w map[string]any
	if e := json.Unmarshal(raw, &w); e != nil || len(w) == 0 || len(w) > 500 {
		return nil, errors.New("Workflow API JSON de 1 à 500 nœuds requis")
	}
	count := 0
	for _, v := range w {
		node, ok := v.(map[string]any)
		if !ok {
			return nil, errors.New("Nœud invalide")
		}
		if typ, ok := node["class_type"].(string); !ok || typ == "" {
			return nil, errors.New("Exporter au format API ComfyUI, pas au format graphique")
		}
		in, ok := node["inputs"].(map[string]any)
		if !ok {
			return nil, errors.New("Entrées de nœud absentes")
		}
		for k, v := range in {
			if str, ok := v.(string); ok && str == "{{PROMPT}}" {
				in[k] = prompt
				count++
			}
		}
	}
	if count == 0 {
		return nil, errors.New("Remplacer le texte positif du workflow API par {{PROMPT}}")
	}
	return w, nil
}

type mediaFile struct {
	Filename  string `json:"filename"`
	Subfolder string `json:"subfolder"`
	Type      string `json:"type"`
}

func fileAllowed(k string) bool {
	return mediaExt[strings.ToLower(filepath.Ext(k))] && portable(k) && !strings.Contains(k, "/") && len(k) < 200
}
func expectedMedia(kind, name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch kind {
	case "image":
		return ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp"
	case "video":
		return ext == ".mp4" || ext == ".webm" || ext == ".gif"
	case "musique":
		return ext == ".wav" || ext == ".mp3" || ext == ".flac" || ext == ".ogg" || ext == ".m4a"
	}
	return false
}
func (s *Studio) copyMedia(ctx context.Context, job string, f mediaFile, n int) (string, error) {
	if f.Type != "output" || !fileAllowed(f.Filename) || (f.Subfolder != "" && !portable(f.Subfolder)) {
		return "", errors.New("Sortie média non autorisée")
	}
	name := fmt.Sprintf("creation-%03d%s", n, strings.ToLower(filepath.Ext(f.Filename)))
	v := url.Values{"filename": {f.Filename}, "subfolder": {f.Subfolder}, "type": {"output"}}
	req, e := http.NewRequestWithContext(ctx, "GET", s.comfy+"/view?"+v.Encode(), nil)
	if e != nil {
		return "", e
	}
	res, e := s.c.Do(req)
	if e != nil {
		return "", e
	}
	defer res.Body.Close()
	if res.StatusCode != 200 || res.ContentLength > maxOutput {
		return "", fmt.Errorf("Média indisponible ou trop volumineux (HTTP %d)", res.StatusCode)
	}
	p := filepath.Join(s.outputDir(job), name)
	out, e := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return "", e
	}
	written, e := io.Copy(out, io.LimitReader(res.Body, maxOutput+1))
	ce := out.Close()
	if e != nil || ce != nil || written == 0 || written > maxOutput {
		os.Remove(p)
		return "", errors.New("Copie média incomplète ou trop grande")
	}
	return name, nil
}
func (s *Studio) media(ctx context.Context, job string, q Request) error {
	w, e := workflow(q.Workflow, q.Prompt)
	if e != nil {
		return e
	}
	var ack struct {
		PromptID string `json:"prompt_id"`
		Error    any    `json:"error"`
	}
	if e = api(ctx, s.c, s.comfy, "/prompt", map[string]any{"prompt": w, "client_id": job}, &ack); e != nil {
		return fmt.Errorf("ComfyUI : %w", e)
	}
	if ack.Error != nil || !promptID.MatchString(ack.PromptID) {
		return errors.New("Workflow refusé ou identifiant moteur invalide")
	}
	s.change(func(j *Job) {
		j.PromptID = ack.PromptID
		j.Text = "Workflow accepté par ComfyUI. En attente des sorties…"
	})
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done(): // Delete only this queued request; never interrupt someone else's running workflow.
			c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = api(c, s.c, s.comfy, "/queue", map[string]any{"delete": []string{ack.PromptID}}, new(any))
			return ctx.Err()
		case <-tick.C:
			var hist map[string]struct {
				Outputs map[string]map[string]json.RawMessage `json:"outputs"`
				Status  struct {
					Completed bool   `json:"completed"`
					StatusStr string `json:"status_str"`
				} `json:"status"`
			}
			if e = api(ctx, s.c, s.comfy, "/history/"+ack.PromptID, nil, &hist); e != nil {
				return e
			}
			h, ok := hist[ack.PromptID]
			if !ok {
				continue
			}
			if h.Status.StatusStr == "error" {
				return errors.New("Échec du workflow : consulter ComfyUI")
			}
			if !h.Status.Completed {
				continue
			}
			if e = os.Mkdir(s.outputDir(job), 0700); e != nil {
				return e
			}
			n := 0
			var total int64
			seen := map[string]bool{}
			for _, node := range h.Outputs {
				for _, key := range []string{"images", "gifs", "audio", "videos", "files"} {
					var files []mediaFile
					if json.Unmarshal(node[key], &files) != nil {
						continue
					}
					for _, f := range files {
						if f.Type != "output" || !fileAllowed(f.Filename) || !expectedMedia(q.Kind, f.Filename) {
							continue
						}
						k := f.Subfolder + "/" + f.Filename
						if seen[k] {
							continue
						}
						seen[k] = true
						n++
						if n > 32 {
							return errors.New("Limite de 32 fichiers par génération")
						}
						name, e := s.copyMedia(ctx, job, f, n)
						if e != nil {
							return e
						}
						st, e := os.Stat(filepath.Join(s.outputDir(job), name))
						if e != nil {
							return e
						}
						total += st.Size()
						if total > 4<<30 {
							os.Remove(filepath.Join(s.outputDir(job), name))
							return errors.New("Limite de 4 Gio par génération dépassée")
						}
						s.change(func(j *Job) { j.Files = append(j.Files, name) })
					}
				}
			}
			if n == 0 {
				return errors.New("Aucune sortie média compatible : ajouter un nœud de sauvegarde au workflow")
			}
			s.change(func(j *Job) { j.Text = "Médias copiés dans le dossier de créations." })
			return nil
		}
	}
}
func validRequest(q Request) error {
	if strings.TrimSpace(q.Prompt) == "" || len(q.Prompt) > 128<<10 {
		return errors.New("Prompt non vide de 128 Kio maximum requis")
	}
	switch q.Kind {
	case "texte", "code":
		if !modelID.MatchString(q.Model) || q.Context < 1024 || q.Context > 32768 || q.Tokens < 64 || q.Tokens > 8192 {
			return errors.New("Modèle ou limites de génération invalides")
		}
		if len(q.History) > 40 {
			return errors.New("Historique limité à 40 messages")
		}
		for _, m := range q.History {
			if (m.Role != "user" && m.Role != "assistant") || len(m.Content) > 128<<10 {
				return errors.New("Message invalide")
			}
		}
	case "image", "video", "musique":
		if !q.Trusted {
			return errors.New("Confirmer que le workflow et ses nœuds sont fiables")
		}
		_, e := workflow(q.Workflow, q.Prompt)
		return e
	default:
		return errors.New("Section inconnue")
	}
	return nil
}
func (s *Studio) observer() Report {
	s.obsMu.Lock()
	defer s.obsMu.Unlock()
	if time.Since(s.obsAt) < 15*time.Second {
		return s.obs
	}
	r := Report{Date: time.Now().Format(time.RFC3339), OS: runtime.GOOS, Arch: runtime.GOARCH}
	gpu(&r)
	s.obs = r
	s.obsAt = time.Now()
	return r
}
func (s *Studio) status(ctx context.Context) any {
	c, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var m []modelInfo
	var oe, ce string
	var stats map[string]any
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		var e error
		m, e = s.localModels(c)
		if e != nil {
			oe = e.Error()
		}
	}()
	go func() {
		defer wg.Done()
		if e := api(c, s.c, s.comfy, "/system_stats", nil, &stats); e != nil {
			ce = e.Error()
		} else if stats["system"] == nil {
			ce = "Réponse moteur inattendue"
		}
	}()
	wg.Wait()
	return map[string]any{"models": m, "ollama_error": oe, "comfy_error": ce, "comfy_stats": stats, "root": s.root}
}
func (s *Studio) serve(w http.ResponseWriter, r *http.Request) {
	h, _, e := net.SplitHostPort(r.Host)
	if e != nil || !(h == "localhost" || h == "127.0.0.1" || h == "::1") {
		reject(w, 403, "Hôte refusé")
		return
	}
	if o := r.Header.Get("Origin"); o != "" && o != "http://"+r.Host {
		reject(w, 403, "Origine refusée")
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' blob:; media-src 'self' blob:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		if r.Method != "GET" && r.Method != "HEAD" {
			reject(w, 405, "Méthode interdite")
			return
		}
		sub, _ := fs.Sub(webFiles, "web")
		http.FileServer(http.FS(sub)).ServeHTTP(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Path == "/api/session" && r.Method == "GET" {
		http.SetCookie(w, &http.Cookie{Name: "studio_session", Value: s.token, Path: "/api/file", HttpOnly: true, SameSite: http.SameSiteStrictMode})
		writeJSON(w, 200, map[string]string{"token": s.token})
		return
	}
	authenticated := subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Studio-Token")), []byte(s.token)) == 1
	if !authenticated && r.Method == "GET" && r.URL.Path == "/api/file" {
		if c, e := r.Cookie("studio_session"); e == nil && r.Header.Get("Sec-Fetch-Site") != "cross-site" {
			authenticated = subtle.ConstantTimeCompare([]byte(c.Value), []byte(s.token)) == 1
		}
	}
	if !authenticated {
		reject(w, 403, "Session invalide")
		return
	}
	switch r.Method + " " + r.URL.Path {
	case "GET /api/status":
		writeJSON(w, 200, s.status(r.Context()))
	case "GET /api/observer":
		writeJSON(w, 200, s.observer())
	case "GET /api/job":
		writeJSON(w, 200, s.snapshot())
	case "POST /api/generate":
		var q Request
		if e := decode(w, r, &q); e != nil {
			reject(w, 400, e.Error())
			return
		}
		if e := validRequest(q); e != nil {
			reject(w, 400, e.Error())
			return
		}
		j, e := s.start(q.Kind, func(ctx context.Context, job string) error {
			if q.Kind == "texte" || q.Kind == "code" {
				return s.chat(ctx, job, q)
			}
			return s.media(ctx, job, q)
		})
		if e != nil {
			reject(w, 409, e.Error())
			return
		}
		writeJSON(w, 202, j)
	case "POST /api/stop":
		s.mu.Lock()
		if s.active() && s.cancel != nil && (s.job.Kind == "texte" || s.job.Kind == "code" || s.job.Kind == "image" || s.job.Kind == "video" || s.job.Kind == "musique" || s.job.Kind == "pull") {
			s.job.State = "stopping"
			s.cancel()
			s.mu.Unlock()
			writeJSON(w, 200, map[string]bool{"requested": true})
		} else {
			s.mu.Unlock()
			reject(w, 409, "Aucune génération arrêtable")
		}
	case "POST /api/backup":
		j, e := s.start("backup", func(ctx context.Context, job string) error {
			p := filepath.Join(s.root, "backups", job+".zip")
			if e := backup(filepath.Join(s.root, "data"), p); e != nil {
				return e
			}
			s.change(func(j *Job) { j.Text = "Sauvegarde vérifiée : " + p })
			return nil
		})
		if e != nil {
			reject(w, 409, e.Error())
			return
		}
		writeJSON(w, 202, j)
	case "POST /api/benchmark":
		j, e := s.start("benchmark", func(ctx context.Context, job string) error {
			rr := Report{Date: time.Now().Format(time.RFC3339), OS: runtime.GOOS, Arch: runtime.GOARCH}
			runBenchmark(&rr, s.ollama, "auto", 3*time.Minute)
			b, e := json.MarshalIndent(rr, "", "  ")
			if e != nil {
				return e
			}
			p := filepath.Join(s.root, "data", "logs", "benchmark-"+job+".json")
			if e = os.WriteFile(p, b, 0600); e != nil {
				return e
			}
			s.change(func(j *Job) { j.Text = string(b) })
			ok := map[string]bool{}
			for _, check := range rr.Checks {
				ok[check.Name] = check.State == "OK"
			}
			if !ok["Génération Texte"] || !ok["Génération Code"] {
				return errors.New("Benchmark incomplet : consulter le rapport")
			}
			return nil
		})
		if e != nil {
			reject(w, 409, e.Error())
			return
		}
		writeJSON(w, 202, j)
	case "POST /api/pull":
		var q struct {
			Model string `json:"model"`
		}
		if e := decode(w, r, &q); e != nil || !pullModelID(q.Model) {
			reject(w, 400, "Nom de modèle Ollama local invalide (ex. qwen2.5:7b)")
			return
		}
		j, e := s.start("pull", func(ctx context.Context, job string) error { return s.pull(ctx, q.Model) })
		if e != nil {
			reject(w, 409, e.Error())
			return
		}
		writeJSON(w, 202, j)
	case "GET /api/connectivity":
		if e := s.checkNetwork(r.Context(), "https://huggingface.co/api/models?limit=1"); e != nil {
			reject(w, 503, e.Error())
			return
		}
		writeJSON(w, 200, map[string]bool{"reachable": true})
	case "GET /api/library":
		v := r.URL.Query()
		tags := map[string]string{"texte": "text-generation", "code": "text-generation", "image": "text-to-image", "video": "text-to-video", "musique": "text-to-audio"}
		tag := tags[v.Get("kind")]
		if tag == "" || len(v.Get("q")) > 200 {
			reject(w, 400, "Recherche invalide")
			return
		}
		q := url.Values{"pipeline_tag": {tag}, "sort": {"downloads"}, "direction": {"-1"}, "limit": {"30"}, "search": {v.Get("q")}}
		c, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(c, "GET", "https://huggingface.co/api/models?"+q.Encode(), nil)
		res, e := s.c.Do(req)
		if e != nil {
			reject(w, 503, networkMessage)
			return
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			reject(w, 502, fmt.Sprintf("Hugging Face indisponible (HTTP %d)", res.StatusCode))
			return
		}
		var models []struct {
			ID        string `json:"id"`
			Downloads int    `json:"downloads"`
			Likes     int    `json:"likes"`
			Gated     any    `json:"gated"`
		}
		if e = json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&models); e != nil {
			reject(w, 502, e.Error())
			return
		}
		writeJSON(w, 200, models)
	case "GET /api/backups":
		ds, e := os.ReadDir(filepath.Join(s.root, "backups"))
		if e != nil {
			reject(w, 500, e.Error())
			return
		}
		rows := []string{}
		for _, d := range ds {
			if d.Type().IsRegular() && strings.HasSuffix(d.Name(), ".zip") && validID.MatchString(strings.TrimSuffix(d.Name(), ".zip")) {
				rows = append(rows, d.Name())
			}
		}
		writeJSON(w, 200, rows)
	case "POST /api/restore":
		var q struct {
			Name string `json:"name"`
		}
		if e := decode(w, r, &q); e != nil || !strings.HasSuffix(q.Name, ".zip") || !validID.MatchString(strings.TrimSuffix(q.Name, ".zip")) {
			reject(w, 400, "Archive invalide")
			return
		}
		p := filepath.Join(s.root, "backups", q.Name)
		st, e := os.Lstat(p)
		if e != nil || !st.Mode().IsRegular() {
			reject(w, 404, "Archive absente")
			return
		}
		j, e := s.start("restore", func(ctx context.Context, job string) error {
			dst := filepath.Join(s.root, "restauration-"+job)
			if e := restore(p, dst); e != nil {
				return e
			}
			s.change(func(j *Job) { j.Text = "Restauration vérifiée sans écrasement : " + dst })
			return nil
		})
		if e != nil {
			reject(w, 409, e.Error())
			return
		}
		writeJSON(w, 202, j)
	case "GET /api/outputs":
		ds, _ := os.ReadDir(filepath.Join(s.root, "data", "outputs"))
		rows := []map[string]string{}
		for i := len(ds) - 1; i >= 0 && len(rows) < 300; i-- {
			d := ds[i]
			if !d.IsDir() || !validID.MatchString(d.Name()) {
				continue
			}
			ff, _ := os.ReadDir(s.outputDir(d.Name()))
			for _, f := range ff {
				if f.Type().IsRegular() && (f.Name() == "reponse.txt" || f.Name() == "conversation.json" || fileAllowed(f.Name())) {
					rows = append(rows, map[string]string{"job": d.Name(), "name": f.Name()})
				}
			}
		}
		writeJSON(w, 200, rows)
	case "GET /api/file":
		job, name := r.URL.Query().Get("job"), r.URL.Query().Get("name")
		if !validID.MatchString(job) || !(name == "reponse.txt" || name == "conversation.json" || fileAllowed(name)) {
			reject(w, 400, "Fichier invalide")
			return
		}
		dir, e := os.Lstat(s.outputDir(job))
		if e != nil || !dir.IsDir() || dir.Mode()&os.ModeSymlink != 0 {
			reject(w, 404, "Dossier absent")
			return
		}
		p := filepath.Join(s.outputDir(job), name)
		st, e := os.Lstat(p)
		if e != nil || !st.Mode().IsRegular() {
			reject(w, 404, "Fichier absent")
			return
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
		http.ServeFile(w, r, p)
	default:
		reject(w, 404, "Route inconnue")
	}
}
func launchBrowser(addr string) {
	switch runtime.GOOS {
	case "windows":
		if p, e := exec.LookPath("firefox.exe"); e == nil {
			if exec.Command(p, addr).Start() == nil {
				return
			}
		}
		_ = exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", addr).Start()
	case "linux":
		if p, e := exec.LookPath("firefox"); e == nil {
			if exec.Command(p, addr).Start() == nil {
				return
			}
		}
		_ = exec.Command("xdg-open", addr).Start()
	}
}
func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "backup", "restore", "verify":
			backupMain()
			return
		case "diagnostic":
			os.Args = append(os.Args[:1], os.Args[2:]...)
			diagnosticMain()
			return
		}
	}
	noBrowser := flag.Bool("no-browser", false, "Ne pas ouvrir de navigateur")
	root := flag.String("root", "", "Dossier de cette édition")
	port := flag.Int("port", 1420, "Port local")
	ollama := flag.String("ollama", "http://127.0.0.1:11434", "Moteur local Ollama")
	comfy := flag.String("comfy", "http://127.0.0.1:8188", "Moteur local ComfyUI")
	flag.Parse()
	if *root == "" {
		p, e := os.Executable()
		if e != nil {
			panic(e)
		}
		*root = filepath.Dir(p)
	}
	p, e := filepath.Abs(*root)
	if e != nil {
		panic(e)
	}
	s, e := newStudio(p, *ollama, *comfy)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	l, e := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", *port))
	if e != nil {
		fmt.Fprintln(os.Stderr, "Port occupé, aucun autre programme n'a été arrêté :", e)
		os.Exit(1)
	}
	addr := fmt.Sprintf("http://localhost:%d/", l.Addr().(*net.TCPAddr).Port)
	fmt.Println("IA Locale Studio — édition de consolidation", addr, "\nDonnées :", p)
	if !*noBrowser {
		launchBrowser(addr)
	}
	srv := &http.Server{Handler: http.HandlerFunc(s.serve), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 90 * time.Second, IdleTimeout: 45 * time.Second, MaxHeaderBytes: 32 << 10}
	if e = srv.Serve(l); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
