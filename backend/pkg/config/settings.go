package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings regroupe les réglages de l'application modifiables depuis l'UI.
// Priorité : variable d'environnement > fichier de config > défauts.
type Settings struct {
	Workers int    `json:"workers"`
	Threads int    `json:"threads"`
	// PlaylistExportDir est le dossier où « Exporter les playlists » écrit
	// les fichiers .m3u8 (dossier Syncthing, iPhone monté en USB…).
	PlaylistExportDir string `json:"playlist_export_dir"`
}

const (
	DefaultWorkers = 4
	DefaultThreads = 6
)

// ConfigPath retourne le chemin du fichier de configuration
// (surchargable via SONEPH_CONFIG).
func ConfigPath() string {
	if p := os.Getenv("SONEPH_CONFIG"); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "soneph", "settings.json")
}

func Load() Settings {
	s := Settings{Workers: DefaultWorkers, Threads: DefaultThreads}
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		return s
	}
	_ = json.Unmarshal(data, &s)
	if s.Workers <= 0 {
		s.Workers = DefaultWorkers
	}
	if s.Threads <= 0 {
		s.Threads = DefaultThreads
	}
	return s
}

func Save(s Settings) error {
	if s.Workers <= 0 {
		s.Workers = DefaultWorkers
	}
	if s.Threads <= 0 {
		s.Threads = DefaultThreads
	}
	path := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
