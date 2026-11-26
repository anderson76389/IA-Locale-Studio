package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"
)

type modelInfo struct {
	Name        string `json:"name"`
	Model       string `json:"model"`
	Size        int64  `json:"size"`
	SizeVRAM    *int64 `json:"size_vram,omitempty"`
	RemoteHost  string `json:"remote_host"`
	RemoteModel string `json:"remote_model"`
	Details     struct {
		Format string `json:"format"`
		Family string `json:"family"`
	} `json:"details"`
}
type modelList struct {
	Models []modelInfo `json:"models"`
}
type generation struct {
	Response     string `json:"response"`
	Done         bool   `json:"done"`
	DoneReason   string `json:"done_reason"`
	Error        string `json:"error"`
	EvalCount    int    `json:"eval_count"`
	EvalDuration int64  `json:"eval_duration"`
	LoadDuration int64  `json:"load_duration"`
}

func modelName(m modelInfo) string {
	if m.Name != "" {
		return m.Name
	}
	return m.Model
}

// Restrict generation to loopback, not LAN or public servers. Never pull models.
func benchmarkBase(raw string) (string, error) {
	u, e := localURL(raw)
	if e != nil {
		return "", e
	}
	ip := net.ParseIP(u.Hostname())
	if !strings.EqualFold(u.Hostname(), "localhost") && (ip == nil || !ip.IsLoopback()) {
		return "", fmt.Errorf("benchmark limité à la boucle locale")
	}
	if u.Path != "" && u.Path != "/" {
		return "", fmt.Errorf("URL de base sans chemin attendue")
	}
	return strings.TrimRight(u.String(), "/"), nil
}
func api(ctx context.Context, c *http.Client, base, path string, body any, out any) error {
	var reader io.Reader
	method := "GET"
	if body != nil {
		b, e := json.Marshal(body)
		if e != nil {
			return e
		}
		reader = bytes.NewReader(b)
		method = "POST"
	}
	req, e := http.NewRequestWithContext(ctx, method, base+path, reader)
	if e != nil {
		return e
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, e := c.Do(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, 4<<20+1))
	if e != nil {
		return e
	}
	if len(b) > 4<<20 {
		return fmt.Errorf("réponse trop grande")
	}
	return json.Unmarshal(b, out)
}
func selectModel(ms []modelInfo, requested string) (modelInfo, error) {
	eligible := []modelInfo{}
	for _, m := range ms {
		n := strings.ToLower(modelName(m))
		if n == "" || m.Size <= 0 || m.Size > 9*1024*1024*1024 || !strings.EqualFold(m.Details.Format, "gguf") || m.RemoteHost != "" || m.RemoteModel != "" || strings.Contains(n, "cloud") || strings.Contains(n, "embed") || strings.Contains(strings.ToLower(m.Details.Family), "bert") {
			continue
		}
		eligible = append(eligible, m)
	}
	sort.Slice(eligible, func(i, j int) bool { return eligible[i].Size < eligible[j].Size })
	for _, m := range eligible {
		if requested == "auto" || modelName(m) == requested {
			return m, nil
		}
	}
	return modelInfo{}, fmt.Errorf("aucun modèle GGUF local compatible trouvé (maximum 9 Gio, sans cloud ni embeddings connus)")
}
func runBenchmark(r *Report, raw, requested string, timeout time.Duration) {
	base, e := benchmarkBase(raw)
	if e != nil {
		r.add("Benchmark Ollama", "NON_TESTE", e.Error())
		return
	}
	c := client()
	c.Timeout = 0
	get := func(path string, out any) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return api(ctx, c, base, path, nil, out)
	}
	var installed, running modelList
	if e = get("/api/tags", &installed); e != nil || installed.Models == nil {
		r.add("Benchmark Ollama", "NON_TESTE", "inventaire local indisponible")
		return
	}
	model, e := selectModel(installed.Models, requested)
	if e != nil {
		r.add("Benchmark Ollama", "NON_TESTE", e.Error())
		return
	}
	if e = get("/api/ps", &running); e != nil || running.Models == nil || len(running.Models) > 0 {
		r.add("Benchmark Ollama", "NON_TESTE", "Ollama doit être accessible et sans modèle déjà chargé; ne pas lancer pendant une autre génération")
		return
	}
	name := modelName(model)
	r.add("Modèle de test", "OK", fmt.Sprintf("%s; %.2f Gio sur disque", name, float64(model.Size)/(1<<30)))
	r.add("Avant génération", "OK", "aucun modèle chargé selon /api/ps")
	cases := []struct{ name, prompt, want string }{{"Texte", "Réponds uniquement avec le nombre obtenu : combien font 17 + 25 ?", "42"}, {"Code", "En Python, écris une fonction add(a, b) qui retourne la somme des arguments. Pas de texte explicatif.", ""}}
	for i, t := range cases {
		fmt.Printf("Test %s : génération en cours...\n", t.name)
		keep := any("2m")
		if i == len(cases)-1 {
			keep = 0
		}
		payload := map[string]any{"model": name, "prompt": t.prompt, "stream": false, "keep_alive": keep, "options": map[string]any{"num_ctx": 2048, "num_predict": 128, "temperature": 0, "seed": 42}}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		start := time.Now()
		var g generation
		e = api(ctx, c, base, "/api/generate", payload, &g)
		elapsed := time.Since(start)
		cancel()
		if e != nil || g.Error != "" || !g.Done || strings.TrimSpace(g.Response) == "" || g.EvalCount <= 0 || g.EvalDuration <= 0 {
			r.add("Génération "+t.name, "NON_VERIFIE", fmt.Sprintf("réponse incomplète ou erreur: %v; %s", e, g.Error))
			break
		}
		detail := fmt.Sprintf("%.2f s murales; %d tokens; %.2f tokens/s; chargement %.2f s; fin %s", elapsed.Seconds(), g.EvalCount, float64(g.EvalCount)/(float64(g.EvalDuration)/1e9), float64(g.LoadDuration)/1e9, g.DoneReason)
		r.add("Génération "+t.name, "OK", detail)
		if t.want != "" {
			if strings.TrimSpace(g.Response) == t.want {
				r.add("Réponse texte attendue", "OK", "42")
			} else {
				r.add("Réponse texte attendue", "A_VERIFIER", "réponse différente de 42 exactement")
			}
		}
		if t.name == "Code" {
			r.add("Validité du code généré", "NON_TESTE", "le code généré n'est jamais exécuté")
		}
		if i == 0 {
			var loaded modelList
			if get("/api/ps", &loaded) == nil {
				found := false
				for _, m := range loaded.Models {
					if modelName(m) == name && m.SizeVRAM != nil {
						r.add("VRAM modèle chargé", "OK", fmt.Sprintf("%.2f Gio selon Ollama (pas le pic GPU)", float64(*m.SizeVRAM)/(1<<30)))
						found = true
					}
				}
				if !found {
					r.add("VRAM modèle chargé", "NON_VERIFIE", "mesure absente")
				}
			}
		}
	}
	// Only observe release. Never unload other jobs/models. keep_alive=0 requests release on final successful call.
	for i := 0; i < 6; i++ {
		var loaded modelList
		if get("/api/ps", &loaded) == nil && loaded.Models != nil {
			found := false
			for _, m := range loaded.Models {
				if modelName(m) == name {
					found = true
				}
			}
			if !found {
				r.add("Libération du modèle", "OK", "modèle absent de /api/ps; ne prouve pas zéro fuite VRAM")
				return
			}
		}
		if i < 5 {
			time.Sleep(time.Second)
		}
	}
	r.add("Libération du modèle", "NON_VERIFIE", "encore chargé ou état inconnu; peut expirer automatiquement après deux minutes")
}
