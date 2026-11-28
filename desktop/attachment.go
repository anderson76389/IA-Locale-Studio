package main

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// Only local executables handle attachments. Temporary inputs are removed on every exit.
func (s *Studio) attachment(w http.ResponseWriter, r *http.Request) {
	dir, e := os.MkdirTemp(s.root, "attachment-")
	if e != nil {
		reject(w, 500, e.Error())
		return
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "input")
	f, e := os.Create(path)
	if e != nil {
		reject(w, 500, e.Error())
		return
	}
	_, e = io.Copy(f, http.MaxBytesReader(w, r.Body, 32<<20))
	closeErr := f.Close()
	if e != nil || closeErr != nil {
		reject(w, 400, "Lecture impossible ou fichier supérieur à 32 Mio")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	out := filepath.Join(dir, "transcription.txt")
	switch strings.ToLower(filepath.Ext(r.URL.Query().Get("name"))) {
	case ".docx", ".pptx":
		value, err := officeText(path, strings.ToLower(filepath.Ext(r.URL.Query().Get("name"))))
		if err != nil {
			reject(w, 422, err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"text": value})
		return
	case ".pdf":
		if e = exec.CommandContext(ctx, "pdftotext", "-enc", "UTF-8", path, out).Run(); e != nil {
			reject(w, 422, "Extraction PDF impossible. Vérifiez pdftotext et la présence de texte dans le PDF.")
			return
		}
	case ".mp3", ".wav", ".m4a", ".ogg", ".flac", ".aac", ".opus", ".webm", ".mp4":
		model := os.Getenv("IA_WHISPER_MODEL")
		if model == "" {
			reject(w, 422, "Définissez IA_WHISPER_MODEL avec le chemin du modèle whisper.cpp local.")
			return
		}
		wav := filepath.Join(dir, "audio.wav")
		if e = exec.CommandContext(ctx, "ffmpeg", "-nostdin", "-v", "error", "-protocol_whitelist", "file,pipe", "-format_whitelist", "mp3,wav,mov,ogg,flac,aac,matroska,webm", "-i", path, "-vn", "-ac", "1", "-ar", "16000", "-t", "600", wav).Run(); e != nil {
			reject(w, 422, "Décodage audio impossible. Vérifiez FFmpeg et le fichier.")
			return
		}
		if e = exec.CommandContext(ctx, "whisper-cli", "-m", model, "-f", wav, "-l", "auto", "-otxt", "-of", strings.TrimSuffix(out, ".txt")).Run(); e != nil {
			reject(w, 422, "Transcription impossible. Vérifiez whisper-cli et le modèle local.")
			return
		}
	default:
		out = path
	}
	source, e := os.Open(out)
	if e != nil {
		reject(w, 422, e.Error())
		return
	}
	defer source.Close()
	b, e := io.ReadAll(io.LimitReader(source, 96<<10+1))
	if e != nil || len(b) > 96<<10 || !utf8.Valid(b) || strings.Contains(string(b), "\x00") || strings.TrimSpace(string(b)) == "" {
		reject(w, 422, "Texte UTF-8 non vide de 96 Kio maximum requis")
		return
	}
	writeJSON(w, 200, map[string]string{"text": string(b)})
}

func officeText(path, ext string) (string, error) {
	archive, e := zip.OpenReader(path)
	if e != nil {
		return "", e
	}
	defer archive.Close()
	var text strings.Builder
	var total uint64
	for _, file := range archive.File {
		selected := file.Name == "word/document.xml" && ext == ".docx"
		if ext == ".pptx" {
			selected = strings.HasPrefix(file.Name, "ppt/slides/slide") && filepath.Ext(file.Name) == ".xml"
		}
		if !selected {
			continue
		}
		total += file.UncompressedSize64
		if total > 8<<20 {
			return "", fmt.Errorf("Document Office trop long")
		}
		src, e := file.Open()
		if e != nil {
			return "", e
		}
		decoder := xml.NewDecoder(io.LimitReader(src, 8<<20))
		var inside bool
		for {
			token, e := decoder.Token()
			if e == io.EOF {
				break
			}
			if e != nil {
				src.Close()
				return "", e
			}
			switch t := token.(type) {
			case xml.StartElement:
				inside = t.Name.Local == "t"
			case xml.CharData:
				if inside {
					text.Write(t)
				}
			case xml.EndElement:
				inside = false
				if t.Name.Local == "p" {
					text.WriteByte('\n')
				}
			}
			if text.Len() > 96<<10 {
				src.Close()
				return "", fmt.Errorf("Document supérieur à 96 Kio de texte")
			}
		}
		src.Close()
	}
	result := strings.TrimSpace(text.String())
	if result == "" {
		return "", fmt.Errorf("Aucun texte dans ce document Office")
	}
	return result, nil
}
