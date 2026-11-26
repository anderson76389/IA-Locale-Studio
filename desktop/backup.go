package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const manifestName = "manifest.json"

type Entry struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
	Dir    bool   `json:"directory,omitempty"`
	Mode   uint32 `json:"mode"`
}
type Manifest struct {
	Version int     `json:"version"`
	Date    string  `json:"date"`
	Entries []Entry `json:"entries"`
}

func fail(s string) error          { return errors.New(s) }
func abs(p string) (string, error) { return filepath.Abs(filepath.Clean(p)) }
func exists(p string) bool         { _, e := os.Lstat(p); return !os.IsNotExist(e) }
func inside(root, p string) bool {
	r, e := filepath.Rel(root, p)
	return e == nil && r != ".." && !strings.HasPrefix(r, ".."+string(os.PathSeparator)) && !filepath.IsAbs(r)
}
func portable(p string) bool {
	if p == "" || p == "." || path.Clean(p) != p || strings.HasPrefix(p, "/") || strings.ContainsAny(p, "\\:\x00<>\"|?*") {
		return false
	}
	for _, s := range strings.Split(p, "/") {
		if s == ".." || s == "." || strings.HasSuffix(s, ".") || strings.HasSuffix(s, " ") {
			return false
		}
		for _, r := range s {
			if r < 32 {
				return false
			}
		}
		base := strings.ToUpper(strings.SplitN(s, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || base == "CONIN$" || base == "CONOUT$" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9') {
			return false
		}
	}
	return true
}
func walk(root string) ([]Entry, error) {
	var entries []Entry
	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if p == root {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("lien ou fichier spécial non pris en charge: %s", p)
		}
		r, e := filepath.Rel(root, p)
		if e != nil {
			return e
		}
		r = filepath.ToSlash(r)
		if !portable(r) {
			return fmt.Errorf("nom non portable: %s", r)
		}
		k := strings.ToLower(r)
		if seen[k] {
			return fmt.Errorf("collision de noms: %s", r)
		}
		seen[k] = true
		en := Entry{Path: r, Size: info.Size(), Dir: info.IsDir(), Mode: uint32(info.Mode().Perm())}
		if en.Dir {
			en.Size = 0
		}
		entries = append(entries, en)
		return nil
	})
	return entries, err
}
func backup(root, dst string) error {
	root, e := abs(root)
	if e != nil {
		return e
	}
	root, e = filepath.EvalSymlinks(root)
	if e != nil {
		return e
	}
	st, e := os.Stat(root)
	if e != nil {
		return e
	}
	if !st.IsDir() {
		return fail("source non dossier")
	}
	dst, e = abs(dst)
	if e != nil {
		return e
	}
	parent, e := filepath.EvalSymlinks(filepath.Dir(dst))
	if e != nil {
		return e
	}
	dst = filepath.Join(parent, filepath.Base(dst))
	if inside(root, dst) {
		return fail("archive interdite dans le dossier source")
	}
	if exists(dst) {
		return fail("archive déjà existante")
	}
	entries, e := walk(root)
	if e != nil {
		return e
	}
	if len(entries) == 0 {
		return fail("dossier source vide")
	}
	f, e := os.CreateTemp(parent, ".ia-backup-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	defer f.Close()
	z := zip.NewWriter(f)
	for i := range entries {
		en := &entries[i]
		name := "data/" + en.Path
		if en.Dir {
			name += "/"
		}
		h := &zip.FileHeader{Name: name, Method: zip.Store}
		if en.Dir {
			h.SetMode(fs.ModeDir | fs.FileMode(en.Mode))
		} else {
			h.SetMode(fs.FileMode(en.Mode))
		}
		w, e := z.CreateHeader(h)
		if e != nil {
			return e
		}
		if en.Dir {
			continue
		}
		p := filepath.Join(root, filepath.FromSlash(en.Path))
		before, e := os.Lstat(p)
		if e != nil {
			return e
		}
		if !before.Mode().IsRegular() {
			return fail("source modifiée")
		}
		r, e := os.Open(p)
		if e != nil {
			return e
		}
		opened, e := r.Stat()
		if e != nil || !os.SameFile(before, opened) {
			r.Close()
			return fail("source remplacée")
		}
		hash := sha256.New()
		n, e := io.Copy(io.MultiWriter(w, hash), r)
		r.Close()
		if e != nil {
			return e
		}
		after, e := os.Lstat(p)
		if e != nil || !os.SameFile(before, after) || n != en.Size || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
			return fmt.Errorf("fichier modifié pendant la sauvegarde: %s", en.Path)
		}
		en.SHA256 = hex.EncodeToString(hash.Sum(nil))
	}
	again, e := walk(root)
	if e != nil {
		return e
	}
	if len(again) != len(entries) {
		return fail("contenu modifié pendant la sauvegarde")
	}
	for i, en := range again {
		if en.Path != entries[i].Path || en.Size != entries[i].Size || en.Dir != entries[i].Dir {
			return fail("contenu modifié pendant la sauvegarde")
		}
	}
	w, e := z.Create(manifestName)
	if e != nil {
		return e
	}
	if e = json.NewEncoder(w).Encode(Manifest{1, time.Now().UTC().Format(time.RFC3339), entries}); e != nil {
		return e
	}
	if e = z.Close(); e != nil {
		return e
	}
	if e = f.Sync(); e != nil {
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if e = verify(tmp, ""); e != nil {
		return e
	}
	return publishFile(tmp, dst)
}

// Hard link publishes atomically without overwriting. On filesystems without hard links,
// use an exclusive create and copy, deleting the incomplete destination on failure.
func publishFile(tmp, dst string) error {
	if e := os.Link(tmp, dst); e == nil {
		return nil
	}
	r, e := os.Open(tmp)
	if e != nil {
		return e
	}
	defer r.Close()
	w, e := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	ok := false
	defer func() {
		w.Close()
		if !ok {
			os.Remove(dst)
		}
	}()
	if _, e = io.Copy(w, r); e != nil {
		return e
	}
	if e = w.Sync(); e != nil {
		return e
	}
	if e = w.Close(); e != nil {
		return e
	}
	ok = true
	return nil
}
func readManifest(z *zip.ReadCloser) (Manifest, map[string]*zip.File, error) {
	m := Manifest{}
	files := map[string]*zip.File{}
	var mf *zip.File
	for _, f := range z.File {
		if _, ok := files[f.Name]; ok {
			return m, nil, fail("entrée ZIP dupliquée")
		}
		files[f.Name] = f
		if f.Name == manifestName {
			mf = f
		}
	}
	if mf == nil || mf.UncompressedSize64 > 64*1024*1024 {
		return m, nil, fail("manifeste absent ou trop volumineux")
	}
	r, e := mf.Open()
	if e != nil {
		return m, nil, e
	}
	b, e := io.ReadAll(io.LimitReader(r, 64*1024*1024+1))
	r.Close()
	if e != nil {
		return m, nil, e
	}
	if len(b) > 64*1024*1024 {
		return m, nil, fail("manifeste trop volumineux")
	}
	if e = json.Unmarshal(b, &m); e != nil {
		return m, nil, e
	}
	if m.Version != 1 || len(m.Entries) == 0 || len(files) != len(m.Entries)+1 {
		return m, nil, fail("manifeste incohérent")
	}
	seen := map[string]bool{}
	dirs := map[string]bool{}
	for _, en := range m.Entries {
		key := strings.ToLower(en.Path)
		if !portable(en.Path) || seen[key] || en.Size < 0 || en.Mode > 0777 {
			return m, nil, fail("chemin ou métadonnées non valides")
		}
		seen[key] = true
		name := "data/" + en.Path
		if en.Dir {
			name += "/"
			dirs[en.Path] = true
			if en.Size != 0 || en.SHA256 != "" {
				return m, nil, fail("dossier invalide")
			}
		} else {
			if h, e := hex.DecodeString(en.SHA256); e != nil || len(h) != 32 {
				return m, nil, fail("empreinte invalide")
			}
		}
		f := files[name]
		if f == nil || f.UncompressedSize64 != uint64(en.Size) || f.FileInfo().IsDir() != en.Dir || (!en.Dir && !f.Mode().IsRegular()) {
			return m, nil, fmt.Errorf("entrée incohérente: %s", en.Path)
		}
	}
	for _, en := range m.Entries {
		p := path.Dir(en.Path)
		if p != "." && !dirs[p] {
			return m, nil, fail("dossier parent absent")
		}
	}
	return m, files, nil
}
func verify(archive, stage string) error {
	z, e := zip.OpenReader(archive)
	if e != nil {
		return e
	}
	defer z.Close()
	m, files, e := readManifest(z)
	if e != nil {
		return e
	}
	sort.SliceStable(m.Entries, func(i, j int) bool { return m.Entries[i].Path < m.Entries[j].Path })
	var dirs []Entry
	for _, en := range m.Entries {
		name := "data/" + en.Path
		target := filepath.Join(stage, filepath.FromSlash(en.Path))
		if en.Dir {
			if stage != "" {
				if e = os.Mkdir(target, 0700); e != nil {
					return e
				}
				dirs = append(dirs, en)
			}
			continue
		}
		f := files[name]
		r, e := f.Open()
		if e != nil {
			return e
		}
		h := sha256.New()
		var w io.Writer = h
		var out *os.File
		if stage != "" {
			out, e = os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if e != nil {
				r.Close()
				return e
			}
			w = io.MultiWriter(h, out)
		}
		n, e := io.Copy(w, io.LimitReader(r, en.Size))
		if e == nil {
			var b [1]byte
			k, er := r.Read(b[:])
			if k != 0 || er != io.EOF {
				e = fail("taille ou CRC ZIP invalide")
			}
		}
		r.Close()
		if out != nil {
			if e2 := out.Close(); e == nil {
				e = e2
			}
		}
		if e != nil {
			return e
		}
		if n != en.Size || hex.EncodeToString(h.Sum(nil)) != en.SHA256 {
			return fmt.Errorf("intégrité invalide: %s", en.Path)
		}
		if stage != "" && runtime.GOOS != "windows" {
			if e = os.Chmod(target, fs.FileMode(en.Mode)); e != nil {
				return e
			}
		}
	}
	if stage != "" && runtime.GOOS != "windows" {
		for i := len(dirs) - 1; i >= 0; i-- {
			d := dirs[i]
			if e = os.Chmod(filepath.Join(stage, filepath.FromSlash(d.Path)), fs.FileMode(d.Mode)); e != nil {
				return e
			}
		}
	}
	return nil
}
func restore(archive, dst string) error {
	dst, e := abs(dst)
	if e != nil {
		return e
	}
	if exists(dst) {
		return fail("destination existante: restauration sans écrasement uniquement")
	}
	parent, e := filepath.EvalSymlinks(filepath.Dir(dst))
	if e != nil {
		return e
	}
	dst = filepath.Join(parent, filepath.Base(dst))
	stage, e := os.MkdirTemp(parent, ".ia-restore-*")
	if e != nil {
		return e
	}
	defer os.RemoveAll(stage)
	if e = verify(archive, stage); e != nil {
		return e
	}
	if exists(dst) {
		return fail("destination créée pendant la restauration")
	} // mkdir reserves the destination, rename moves into a new child without replacing any existing data.
	if e = os.Mkdir(dst, 0700); e != nil {
		return e
	}
	result := filepath.Join(dst, "IA-Locale-Studio")
	if e = os.Rename(stage, result); e != nil {
		os.Remove(dst)
		return e
	}
	return nil
}
func run(args []string) error {
	if len(args) == 3 && args[0] == "backup" {
		return backup(args[1], args[2])
	}
	if len(args) == 2 && args[0] == "verify" {
		return verify(args[1], "")
	}
	if len(args) == 3 && args[0] == "restore" {
		return restore(args[1], args[2])
	}
	return fail("Usage: backup DOSSIER ARCHIVE.zip | verify ARCHIVE.zip | restore ARCHIVE.zip NOUVEAU_DOSSIER")
}
func backupMain() {
	if e := run(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, "ERREUR:", e)
		os.Exit(1)
	}
	fmt.Println("OK : opération terminée et intégrité vérifiée.")
}
