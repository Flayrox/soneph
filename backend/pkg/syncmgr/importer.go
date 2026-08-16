package syncmgr

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
)

// Status décrit l'état de l'auto-import pour l'UI.
type Status struct {
	Available     bool   `json:"available"`
	Running       bool   `json:"running"`
	Platform      string `json:"platform"`
	DownloadsDir  string `json:"downloads_dir"`
	AutoAddDir    string `json:"auto_add_dir,omitempty"`
	ImportedCount int    `json:"imported_count"`
	Pid           int    `json:"pid,omitempty"`
	StateFile     string `json:"state_file"`
	LogFile       string `json:"log_file"`
	Error         string `json:"error,omitempty"`
}

// Importer pilote scripts/watch_and_import.sh (copie des nouveaux fichiers
// audio vers « Automatically Add to Music » — voir le script pour le détail).
type Importer struct {
	mu           sync.Mutex
	downloadsDir string
	scriptPath   string
	stateFile    string
	logFile      string
	pidFile      string
	pid          int
}

func findScript() string {
	for _, p := range []string{
		"../scripts/watch_and_import.sh",
		"scripts/watch_and_import.sh",
		"/app/scripts/watch_and_import.sh",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func findAutoAddDir() string {
	home, _ := os.UserHomeDir()
	for _, p := range []string{
		filepath.Join(home, "Music", "Music", "Media.localized", "Automatically Add to Music.localized"),
		filepath.Join(home, "Music", "Music", "Automatically Add to Music"),
		filepath.Join(home, "Music", "iTunes", "iTunes Media", "Automatically Add to Music"),
		filepath.Join(home, "Music", "iTunes", "iTunes Media", "Automatically Add to iTunes"),
	} {
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return p
		}
	}
	return ""
}

func New(downloadsDir string) *Importer {
	home, _ := os.UserHomeDir()
	stateDir := filepath.Join(home, ".soneph")
	return &Importer{
		downloadsDir: downloadsDir,
		scriptPath:   findScript(),
		stateFile:    filepath.Join(stateDir, "imported.txt"),
		logFile:      filepath.Join(stateDir, "watcher.log"),
		pidFile:      filepath.Join(stateDir, "watcher.pid"),
	}
}

func isAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

func readPid(pidFile string) int {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0
	}
	var pid int
	_, _ = fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &pid)
	return pid
}

func (m *Importer) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()

	st := Status{
		Platform:     runtime.GOOS,
		DownloadsDir: m.downloadsDir,
		StateFile:    m.stateFile,
		LogFile:      m.logFile,
	}
	if runtime.GOOS != "darwin" {
		st.Error = "L'auto-import n'est disponible que sur macOS (l'app Musique doit être installée)."
		return st
	}
	if m.scriptPath == "" {
		st.Error = "scripts/watch_and_import.sh introuvable."
		return st
	}
	st.Available = true
	st.AutoAddDir = findAutoAddDir()
	if st.AutoAddDir == "" {
		st.Error = "Dossier « Automatically Add to Music » introuvable — ouvre l'app Musique une fois, puis réessaie."
	}

	pid := m.pid
	if pid == 0 {
		pid = readPid(m.pidFile)
	}
	st.Pid = pid
	st.Running = isAlive(pid)

	if f, err := os.Open(m.stateFile); err == nil {
		defer f.Close()
		n := 0
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			n++
		}
		st.ImportedCount = n
	}
	return st
}

func (m *Importer) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if runtime.GOOS != "darwin" {
		return fmt.Errorf("l'auto-import n'est disponible que sur macOS")
	}
	if m.scriptPath == "" {
		return fmt.Errorf("scripts/watch_and_import.sh introuvable")
	}
	if m.pid > 0 && isAlive(m.pid) {
		return fmt.Errorf("le watcher est déjà en cours d'exécution (pid %d)", m.pid)
	}
	if err := os.MkdirAll(filepath.Dir(m.logFile), 0o755); err != nil {
		return fmt.Errorf("impossible de créer le dossier d'état : %v", err)
	}

	logFile, err := os.OpenFile(m.logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("impossible d'ouvrir le log : %v", err)
	}

	cmd := exec.Command("bash", m.scriptPath)
	cmd.Env = append(os.Environ(), "SONEPH_DOWNLOADS="+m.downloadsDir)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("échec du démarrage du watcher : %v", err)
	}
	m.pid = cmd.Process.Pid
	_ = os.WriteFile(m.pidFile, []byte(fmt.Sprintf("%d\n", m.pid)), 0o644)
	return nil
}

func (m *Importer) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pid := m.pid
	if pid == 0 {
		pid = readPid(m.pidFile)
	}
	if pid <= 0 || !isAlive(pid) {
		_ = os.Remove(m.pidFile)
		m.pid = 0
		return nil
	}
	// TERM au groupe entier (bash + fswatch) pour un arrêt propre
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	_ = syscall.Kill(pid, syscall.SIGTERM)
	_ = os.Remove(m.pidFile)
	m.pid = 0
	return nil
}
