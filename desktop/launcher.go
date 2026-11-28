package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func installLinuxLauncher() error {
	if runtime.GOOS != "linux" {
		return errors.New("Le lanceur de bureau est disponible sous Linux.")
	}
	base := os.Getenv("XDG_DATA_HOME")
	if !filepath.IsAbs(base) {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		base = filepath.Join(home, ".local", "share")
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	if strings.ContainsAny(executable, "\r\n") {
		return errors.New("Le chemin de l’exécutable contient un saut de ligne.")
	}
	icon, err := webFiles.ReadFile("web/studio-icon.svg")
	if err != nil {
		return err
	}
	icons := filepath.Join(base, "icons", "hicolor", "scalable", "apps")
	apps := filepath.Join(base, "applications")
	for _, dir := range []string{icons, apps} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(icons, "fr.ialocale.studio.svg"), icon, 0644); err != nil {
		return err
	}
	// Escape the quoted Exec argument, then escape backslashes for the desktop entry.
	quoted := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "$", "\\$", "`", "\\`", "%", "%%").Replace(executable)
	entry := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=IA Locale Studio\nExec=\"%s\"\nIcon=fr.ialocale.studio\nTerminal=false\nCategories=Utility;\n", strings.ReplaceAll(quoted, "\\", "\\\\"))
	return os.WriteFile(filepath.Join(apps, "fr.ialocale.studio.desktop"), []byte(entry), 0644)
}
