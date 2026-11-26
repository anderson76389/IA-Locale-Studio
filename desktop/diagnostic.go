package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Check struct {
	Name   string `json:"controle"`
	State  string `json:"etat"`
	Detail string `json:"detail"`
}
type Report struct {
	Date   string  `json:"date"`
	OS     string  `json:"os"`
	Arch   string  `json:"architecture"`
	Checks []Check `json:"controles"`
}

func (r *Report) add(n, s, d string) {
	r.Checks = append(r.Checks, Check{n, s, d})
	fmt.Printf("[%s] %s : %s\n", s, n, d)
}
func command(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	b, e := exec.CommandContext(ctx, name, args...).Output()
	return strings.TrimSpace(string(b)), e
}
func client() *http.Client {
	return &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{DialContext: (&net.Dialer{Timeout: 3 * time.Second}).DialContext}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
func localURL(s string) (*url.URL, error) {
	u, e := url.Parse(s)
	if e != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("URL HTTP(S) sans identifiants, requête ni fragment attendue")
	}
	h := u.Hostname()
	ip := net.ParseIP(h)
	if !strings.EqualFold(h, "localhost") && (ip == nil || (!ip.IsLoopback() && !ip.IsPrivate() && !ip.IsLinkLocalUnicast())) {
		return nil, fmt.Errorf("adresse locale ou IP privée uniquement")
	}
	return u, nil
}
func probe(c *http.Client, raw, key string) (string, string) {
	u, e := localURL(raw)
	if e != nil {
		return "ERREUR", e.Error()
	}
	resp, e := c.Get(u.String())
	if e != nil {
		return "NON_VERIFIE", e.Error()
	}
	defer resp.Body.Close()
	b, e := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if e != nil {
		return "ERREUR", e.Error()
	}
	d := fmt.Sprintf("HTTP %d", resp.StatusCode)
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		d += fmt.Sprintf("; TLS vérifié; SHA-256 %x", sha256.Sum256(resp.TLS.PeerCertificates[0].Raw))
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return "PROTEGE", d
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "NON_VERIFIE", d
	}
	if key == "html" {
		s := strings.ToLower(string(b))
		if !strings.Contains(s, "<html") && !strings.Contains(s, "<!doctype html") {
			return "NON_VERIFIE", d + "; HTML absent"
		}
	} else {
		var m map[string]json.RawMessage
		if json.Unmarshal(b, &m) != nil || len(m[key]) == 0 || string(m[key]) == "null" {
			return "NON_VERIFIE", d + "; réponse inattendue"
		}
	}
	return "OK", d
}
func gpu(r *Report) {
	if s, e := command("nvidia-smi", "--query-gpu=name,memory.total,memory.used,utilization.gpu,temperature.gpu,driver_version", "--format=csv,noheader,nounits"); e == nil && s != "" {
		r.add("NVIDIA (nom, VRAM totale/utilisée MiB, charge %, °C, pilote)", "OK", s)
	} else {
		r.add("NVIDIA SMI", "NON_VERIFIE", "outil ou GPU indisponible")
	}
	if runtime.GOOS == "windows" {
		for _, p := range []struct{ n, c string }{{"Adaptateurs vidéo", "Get-CimInstance Win32_VideoController | Select-Object Name,DriverVersion | ConvertTo-Json -Compress"}, {"RAM système (KiB)", "Get-CimInstance Win32_OperatingSystem | Select-Object TotalVisibleMemorySize,FreePhysicalMemory | ConvertTo-Json -Compress"}} {
			s, e := command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", p.c)
			if e == nil && s != "" {
				r.add(p.n, "OK", s)
			} else {
				r.add(p.n, "NON_VERIFIE", "inventaire indisponible")
			}
		}
	} else if runtime.GOOS == "linux" {
		ps, _ := filepath.Glob("/sys/class/drm/card*/device/vendor")
		found := false
		for _, p := range ps {
			b, _ := os.ReadFile(p)
			if strings.TrimSpace(string(b)) != "0x1002" {
				continue
			}
			found = true
			var a []string
			for _, n := range []string{"device", "mem_info_vram_total", "mem_info_vram_used", "gpu_busy_percent"} {
				if v, e := os.ReadFile(filepath.Join(filepath.Dir(p), n)); e == nil {
					a = append(a, n+"="+strings.TrimSpace(string(v)))
				}
			}
			r.add("AMD sysfs (VRAM octets)", "OK", strings.Join(a, "; "))
		}
		if !found {
			r.add("AMD sysfs", "NON_VERIFIE", "aucun adaptateur AMD exposé")
		}
		if b, e := os.ReadFile("/proc/meminfo"); e == nil {
			var a []string
			for _, l := range strings.Split(string(b), "\n") {
				if strings.HasPrefix(l, "MemTotal:") || strings.HasPrefix(l, "MemAvailable:") {
					a = append(a, l)
				}
			}
			r.add("RAM système", "OK", strings.Join(a, "; "))
		}
	}
}
func sdk(r *Report) {
	for _, n := range []string{"java", "adb", "gradle", "sdkmanager"} {
		if _, e := exec.LookPath(n); e == nil {
			r.add(n, "OK", "présent dans PATH (compatibilité non testée)")
		} else {
			r.add(n, "NON_VERIFIE", "absent du PATH")
		}
	}
	root := os.Getenv("ANDROID_HOME")
	if root == "" {
		root = os.Getenv("ANDROID_SDK_ROOT")
	}
	if root == "" {
		if h, e := os.UserHomeDir(); e == nil {
			if runtime.GOOS == "windows" {
				root = filepath.Join(h, "AppData", "Local", "Android", "Sdk")
			} else {
				root = filepath.Join(h, "Android", "Sdk")
			}
		}
	}
	for _, n := range []string{"platforms", "build-tools", "ndk", "platform-tools"} {
		if root != "" {
			if d, e := os.ReadDir(filepath.Join(root, n)); e == nil && len(d) > 0 {
				r.add("SDK Android / "+n, "OK", fmt.Sprintf("%d entrée(s)", len(d)))
				continue
			}
		}
		r.add("SDK Android / "+n, "NON_VERIFIE", "non trouvé")
	}
}
func diagnosticMain() {
	studio := flag.String("studio", "http://localhost:1420/", "URL locale Studio")
	ollama := flag.String("ollama", "http://127.0.0.1:11434", "URL Ollama")
	comfy := flag.String("comfy", "http://127.0.0.1:8188", "URL ComfyUI")
	out := flag.String("out", "", "rapport JSON")
	bench := flag.String("bench", "", "test de génération explicite: auto ou nom exact du modèle")
	timeout := flag.Duration("bench-timeout", 3*time.Minute, "délai maximal par génération")
	flag.Parse()
	r := Report{Date: time.Now().Format(time.RFC3339), OS: runtime.GOOS, Arch: runtime.GOARCH}
	fmt.Println("IA Locale Studio — validation indépendante (génération uniquement sur demande)")
	c := client()
	for _, p := range []struct{ n, u, k string }{{"Studio : interface", *studio, "html"}, {"Ollama : inventaire modèles", strings.TrimRight(*ollama, "/") + "/api/tags", "models"}, {"ComfyUI : système", strings.TrimRight(*comfy, "/") + "/system_stats", "system"}} {
		s, d := probe(c, p.u, p.k)
		r.add(p.n, s, d)
	}
	gpu(&r)
	sdk(&r)
	if *bench != "" && *timeout > 0 && *timeout <= 10*time.Minute {
		runBenchmark(&r, *ollama, *bench, *timeout)
	} else {
		r.add("Génération Texte/Code", "NON_TESTE", "activer -bench auto pour lancer le test")
	}
	r.add("Image / Vidéo / Musique / association mobile / pause-reprise", "NON_TESTE", "aucune génération média ni action authentifiée lancée")
	if *out == "" {
		p, e := os.Executable()
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		*out = filepath.Join(filepath.Dir(p), "diagnostic-"+time.Now().Format("20060102-150405")+".json")
	}
	b, e := json.MarshalIndent(r, "", "  ")
	if e == nil {
		f, err := os.OpenFile(*out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		e = err
		if e == nil {
			_, e = f.Write(append(b, '\n'))
			ce := f.Close()
			if e == nil {
				e = ce
			}
		}
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, "Rapport non enregistré :", e)
		os.Exit(1)
	}
	fmt.Println("Rapport :", *out)
}
