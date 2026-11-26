package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setup(t *testing.T) (string, string) {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "source")
	if e := os.MkdirAll(filepath.Join(root, "models", "vide"), 0700); e != nil {
		t.Fatal(e)
	}
	for p, s := range map[string]string{"config.json": "{\"test\":true}", "models/modèle.gguf": strings.Repeat("abc", 10000), "empty": ""} {
		if e := os.WriteFile(filepath.Join(root, p), []byte(s), 0600); e != nil {
			t.Fatal(e)
		}
	}
	return root, filepath.Join(base, "archive.zip")
}
func TestRoundTrip(t *testing.T) {
	r, a := setup(t)
	if e := backup(r, a); e != nil {
		t.Fatal(e)
	}
	if e := verify(a, ""); e != nil {
		t.Fatal(e)
	}
	d := filepath.Join(filepath.Dir(r), "restore")
	if e := restore(a, d); e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{"config.json", "models/modèle.gguf", "empty"} {
		x, _ := os.ReadFile(filepath.Join(r, p))
		y, e := os.ReadFile(filepath.Join(d, "IA-Locale-Studio", p))
		if e != nil || !bytes.Equal(x, y) {
			t.Fatalf("bad file %s %v", p, e)
		}
	}
	if _, e := os.Stat(filepath.Join(d, "IA-Locale-Studio", "models", "vide")); e != nil {
		t.Fatal(e)
	}
}
func TestNoOverwrite(t *testing.T) {
	r, a := setup(t)
	os.WriteFile(a, []byte("keep"), 0600)
	if backup(r, a) == nil {
		t.Fatal("overwrote archive")
	}
	b, _ := os.ReadFile(a)
	if string(b) != "keep" {
		t.Fatal("changed archive")
	}
	if restore(a, r) == nil {
		t.Fatal("overwrote source")
	}
}
func TestInside(t *testing.T) {
	r, _ := setup(t)
	if backup(r, filepath.Join(r, "archive.zip")) == nil {
		t.Fatal("accepted inside")
	}
}
func TestSymlink(t *testing.T) {
	r, a := setup(t)
	if e := os.Symlink(filepath.Join(r, "config.json"), filepath.Join(r, "link")); e != nil {
		t.Skip(e)
	}
	if backup(r, a) == nil {
		t.Fatal("accepted symlink")
	}
}
func TestPortableNames(t *testing.T) {
	for _, p := range []string{"../file", "/root", "C:/x", "a\\b", "a/../b", "CON.txt", "dir/NUL", "a.", "a ", "foo:bar", "dir//x"} {
		if portable(p) {
			t.Fatalf("accepted %q", p)
		}
	}
	if !portable("modèles/abc.gguf") {
		t.Fatal("unicode rejected")
	}
}
func crafted(t *testing.T, name, hash string, extra bool) string {
	t.Helper()
	a := filepath.Join(t.TempDir(), "bad.zip")
	f, e := os.Create(a)
	if e != nil {
		t.Fatal(e)
	}
	z := zip.NewWriter(f)
	w, e := z.Create("data/" + name)
	if e != nil {
		t.Fatal(e)
	}
	w.Write([]byte("hello"))
	w, _ = z.Create(manifestName)
	json.NewEncoder(w).Encode(Manifest{Version: 1, Entries: []Entry{{Path: name, Size: 5, SHA256: hash, Mode: 0600}}})
	if extra {
		w, _ = z.Create("surprise")
		w.Write([]byte("x"))
	}
	if e = z.Close(); e != nil {
		t.Fatal(e)
	}
	f.Close()
	return a
}
func TestWrongHash(t *testing.T) {
	a := crafted(t, "x", strings.Repeat("0", 64), false)
	if verify(a, "") == nil {
		t.Fatal("accepted invalid hash")
	}
	d := filepath.Join(filepath.Dir(a), "restore")
	if restore(a, d) == nil || exists(d) {
		t.Fatal("failed restore published data")
	}
}
func TestTraversal(t *testing.T) {
	s := sha256.Sum256([]byte("hello"))
	a := crafted(t, "../outside", hex.EncodeToString(s[:]), false)
	if verify(a, "") == nil {
		t.Fatal("accepted traversal")
	}
}
func TestExtraEntry(t *testing.T) {
	s := sha256.Sum256([]byte("hello"))
	a := crafted(t, "x", hex.EncodeToString(s[:]), true)
	if verify(a, "") == nil {
		t.Fatal("accepted extra entry")
	}
}
func TestCorruption(t *testing.T) {
	r, a := setup(t)
	if e := backup(r, a); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(a)
	if e != nil {
		t.Fatal(e)
	}
	i := bytes.Index(b, []byte(strings.Repeat("abc", 100)))
	if i < 0 {
		t.Fatal("data not found")
	}
	b[i] ^= 1
	if e = os.WriteFile(a, b, 0600); e != nil {
		t.Fatal(e)
	}
	if verify(a, "") == nil {
		t.Fatal("accepted corrupted bytes")
	}
}
func TestCollision(t *testing.T) {
	r, a := setup(t)
	if e := os.WriteFile(filepath.Join(r, "CONFIG.JSON"), []byte("x"), 0600); e != nil {
		t.Skip(e)
	}
	if _, e := os.Stat(filepath.Join(r, "config.json")); e != nil {
		t.Skip(e)
	}
	if backup(r, a) == nil {
		t.Fatal("accepted case collision")
	}
}
