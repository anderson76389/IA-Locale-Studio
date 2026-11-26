package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const networkMessage = "Connexion Internet indisponible ou service inaccessible. Connectez-vous à Internet pour télécharger des modèles. Les modèles déjà installés restent utilisables hors ligne."

var pullPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,79}(/[a-zA-Z0-9][a-zA-Z0-9_.-]{0,79})?(:[a-zA-Z0-9][a-zA-Z0-9_.-]{0,79})?$`)

func pullModelID(name string) bool {
	n := strings.ToLower(name)
	return len(name) <= 180 && pullPattern.MatchString(name) && !strings.Contains(n, "cloud") && !strings.Contains(n, "..") && !(strings.Contains(n, "/") && strings.Contains(strings.SplitN(n, "/", 2)[0], "."))
}
func (s *Studio) checkNetwork(ctx context.Context, endpoint string) error {
	c, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, e := http.NewRequestWithContext(c, "GET", endpoint, nil)
	if e != nil {
		return e
	}
	res, e := s.c.Do(req)
	if e != nil {
		return errors.New(networkMessage)
	}
	defer res.Body.Close()
	if res.StatusCode == 200 || (endpoint == "https://registry.ollama.ai/v2/" && res.StatusCode == 401) {
		return nil
	}
	return fmt.Errorf("Service de modèles inaccessible (HTTP %d). Les modèles locaux restent utilisables.", res.StatusCode)
}
func (s *Studio) pull(ctx context.Context, name string) error {
	if !pullModelID(name) {
		return errors.New("Modèle non autorisé")
	}
	// Aucune requête Internet avant l'action explicite de l'utilisateur.
	s.change(func(j *Job) { j.Text = "Vérification de l’accès au registre Ollama…" })
	if e := s.checkNetwork(ctx, "https://registry.ollama.ai/v2/"); e != nil {
		return e
	}
	b, _ := json.Marshal(map[string]any{"model": name, "stream": true})
	req, e := http.NewRequestWithContext(ctx, "POST", s.ollama+"/api/pull", bytes.NewReader(b))
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/json")
	res, e := s.c.Do(req)
	if e != nil {
		return fmt.Errorf("Ollama inaccessible : %w", e)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("Téléchargement Ollama refusé (HTTP %d). Vérifiez le modèle et la connexion Internet.", res.StatusCode)
	}
	scan := bufio.NewScanner(res.Body)
	scan.Buffer(make([]byte, 4096), 1<<20)
	for scan.Scan() {
		var v struct {
			Status    string `json:"status"`
			Error     string `json:"error"`
			Total     int64  `json:"total"`
			Completed int64  `json:"completed"`
		}
		if e := json.Unmarshal(scan.Bytes(), &v); e != nil {
			return e
		}
		if v.Error != "" {
			return fmt.Errorf("Téléchargement interrompu : %s. Vérifiez votre connexion Internet.", v.Error)
		}
		s.change(func(j *Job) {
			j.Text = v.Status
			j.Progress = ""
			if v.Total > 0 && v.Completed >= 0 && v.Completed <= v.Total {
				j.Progress = fmt.Sprintf("%.1f %% du fichier en cours", float64(v.Completed)/float64(v.Total)*100)
			}
		})
		if v.Status == "success" {
			ms, e := s.localModels(ctx)
			if e != nil {
				return e
			}
			want := name
			if !strings.Contains(want, ":") {
				want += ":latest"
			}
			for _, m := range ms {
				if modelName(m) == name || modelName(m) == want {
					s.change(func(j *Job) {
						j.Text = "Modèle local installé : " + modelName(m) + ". Utilisable sans Internet."
						j.Progress = ""
					})
					return nil
				}
			}
			return errors.New("Téléchargement annoncé terminé, mais modèle GGUF local non détecté.")
		}
	}
	if e := scan.Err(); e != nil {
		return fmt.Errorf("Flux de téléchargement interrompu : %w. Vérifiez Internet.", e)
	}
	return errors.New("Téléchargement interrompu avant confirmation. Vérifiez votre connexion Internet.")
}
