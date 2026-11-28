package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type historyTurn struct {
	ID     string `json:"id"`
	Thread string `json:"thread"`
	Kind   string `json:"kind"`
	Prompt string `json:"prompt"`
	Date   int64  `json:"date"`
}

func (s *Studio) turns() ([]historyTurn, error) {
	dirs, err := os.ReadDir(filepath.Join(s.root, "data", "outputs"))
	if err != nil {
		return nil, err
	}
	rows := []historyTurn{}
	for _, d := range dirs {
		if !d.IsDir() || !validID.MatchString(d.Name()) {
			continue
		}
		dir := s.outputDir(d.Name())
		var row historyTurn
		raw, err := os.ReadFile(filepath.Join(dir, "turn.json"))
		if err == nil {
			if json.Unmarshal(raw, &row) != nil {
				continue
			}
		} else {
			// Older versions wrote conversations and creations without a history index.
			var old struct {
				Kind     string
				Messages []Message
			}
			if raw, e := os.ReadFile(filepath.Join(dir, "conversation.json")); e == nil && json.Unmarshal(raw, &old) == nil {
				row.Kind = old.Kind
				for _, m := range old.Messages {
					if m.Role == "user" {
						row.Prompt = m.Content
					}
				}
			} else {
				files, _ := os.ReadDir(dir)
				for _, f := range files {
					if !fileAllowed(f.Name()) {
						continue
					}
					switch strings.ToLower(filepath.Ext(f.Name())) {
					case ".jpg", ".jpeg", ".png", ".webp":
						row.Kind = "image"
					case ".mp4", ".webm", ".gif":
						row.Kind = "video"
					default:
						row.Kind = "musique"
					}
					row.Prompt = f.Name()
					break
				}
			}
			info, e := d.Info()
			if e != nil || row.Kind == "" {
				continue
			}
			row.Thread = d.Name()
			row.Date = info.ModTime().UnixMilli()
		}
		row.ID = d.Name()
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Date < rows[j].Date })
	return rows, nil
}
func (s *Studio) historyAPI(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, e := s.turns()
	if e != nil {
		reject(w, 500, e.Error())
		return
	}
	if r.Method == "GET" {
		writeJSON(w, 200, rows)
		return
	}
	var q struct{ Thread string }
	if decode(w, r, &q) != nil || !validID.MatchString(q.Thread) {
		reject(w, 400, "Discussion invalide")
		return
	}
	if s.active() {
		reject(w, 409, "Attendez la fin de l’opération")
		return
	}
	for _, row := range rows {
		if row.Thread == q.Thread {
			if e = os.RemoveAll(s.outputDir(row.ID)); e != nil {
				reject(w, 500, e.Error())
				return
			}
			if s.job.ID == row.ID {
				s.job = Job{}
			}
		}
	}
	writeJSON(w, 200, true)
}
func (s *Studio) saveTurn(job string, q Request) error {
	thread := q.Thread
	if thread == "" {
		thread = job
	}
	row := historyTurn{job, thread, q.Kind, q.Prompt, s.snapshot().Started.UnixMilli()}
	raw, e := json.Marshal(row)
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(s.outputDir(job), "turn.json"), raw, 0600)
}
func (s *Studio) resume(q *Request) error {
	if q.Thread == "" {
		return nil
	}
	rows, e := s.turns()
	if e != nil {
		return e
	}
	for i := len(rows) - 1; i >= 0; i-- {
		row := rows[i]
		if row.Thread != q.Thread || row.Kind != q.Kind {
			continue
		}
		if q.Kind == "texte" || q.Kind == "code" {
			raw, e := os.ReadFile(filepath.Join(s.outputDir(row.ID), "conversation.json"))
			if e != nil {
				return e
			}
			var chat struct{ Messages []Message }
			if e = json.Unmarshal(raw, &chat); e != nil {
				return e
			}
			q.History = nil
			for _, m := range chat.Messages {
				if m.Role != "system" {
					q.History = append(q.History, m)
				}
			}
			if len(q.History) > 20 {
				q.History = q.History[len(q.History)-20:]
			}
		}
		return nil
	}
	return errors.New("Discussion absente ou atelier différent")
}
