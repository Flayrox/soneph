package downloader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Les regex qui lisent la sortie console du moteur de téléchargement sont
// le point le plus fragile du pipeline (le moteur change son format entre
// versions). Ces tests verrouillent le format qu'on sait parser.
func TestSpotdlLineParsing(t *testing.T) {
	cases := []struct {
		name     string
		line     string
		re       func(string) (string, bool)
		expected string
	}{
		{
			name: "total found",
			line: "Found 42 songs in 1:02:34",
			re: func(l string) (string, bool) {
				m := reTotal.FindStringSubmatch(l)
				if len(m) > 1 {
					return m[1], true
				}
				return "", false
			},
			expected: "42",
		},
		{
			name: "downloaded with quotes",
			line: `Downloaded "Goutte d'eau"`,
			re: func(l string) (string, bool) {
				m := reDownloaded.FindStringSubmatch(l)
				if len(m) > 1 {
					return m[1], true
				}
				return "", false
			},
			expected: "Goutte d'eau",
		},
		{
			name: "downloading track",
			line: `Downloading "La puerta"`,
			re: func(l string) (string, bool) {
				m := reDownloading.FindStringSubmatch(l)
				if len(m) > 1 {
					return m[1], true
				}
				return "", false
			},
			expected: "La puerta",
		},
		{
			name: "skipping existing",
			line: `Skipping Putana (already downloaded)`,
			re: func(l string) (string, bool) {
				m := reSkipping.FindStringSubmatch(l)
				if len(m) > 1 {
					return m[1], true
				}
				return "", false
			},
			expected: "Putana ",
		},
		{
			name: "metadata upgraded single to album",
			line: `Updated metadata for Goutte d'eau - Ninho, moved to new location: /downloads/Ninho/Goutte d'eau/Goutte d'eau.mp3.mp3`,
			re: func(l string) (string, bool) {
				m := reMetadataUpgraded.FindStringSubmatch(l)
				if len(m) > 1 {
					return strings.TrimSpace(m[1]), true
				}
				return "", false
			},
			expected: "Goutte d'eau - Ninho",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := tc.re(tc.line)
			if !ok {
				t.Fatalf("no match for line %q", tc.line)
			}
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := tc.re(tc.line)
			if !ok {
				t.Fatalf("no match for line %q", tc.line)
			}
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

// TestEngineCandidateDirs vérifie que la recherche du moteur couvre les
// emplacements usuels — pipx, Homebrew, pyenv, conda et pip --user macOS
// (~/Library/Python/<3.x>/bin) — même quand ils sont hors PATH (app lancée
// depuis le Finder).
func TestEngineCandidateDirs(t *testing.T) {
	home := t.TempDir()
	for _, v := range []string{"3.12", "3.11"} {
		if err := os.MkdirAll(filepath.Join(home, "Library", "Python", v, "bin"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	dirs := engineCandidateDirsFor(home)
	joined := strings.Join(dirs, "\n")
	for _, want := range []string{
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".pyenv", "shims"),
		filepath.Join(home, "miniconda3", "bin"),
		filepath.Join(home, "anaconda3", "bin"),
		"/opt/homebrew/bin",
		"/usr/local/bin",
		filepath.Join(home, "Library", "Python", "3.12", "bin"),
		filepath.Join(home, "Library", "Python", "3.11", "bin"),
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("candidate %q absent des dossiers cherchés:\n%s", want, joined)
		}
	}

	// Aucun doublon (les entrées macOS sont triées par os.ReadDir).
	seen := map[string]bool{}
	for _, d := range dirs {
		if seen[d] {
			t.Errorf("dossier en double : %s", d)
		}
		seen[d] = true
	}
}

// TestEngineMissingMessage vérifie que le message d'erreur affiché à
// l'utilisateur explique comment installer le moteur, au lieu du vague
// « executable file not found in $PATH ».
func TestEngineMissingMessage(t *testing.T) {
	msg := engineMissingMessage()
	for _, want := range []string{"pipx install spotdl", "pip install spotdl", "spotdl", "Cherché dans"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message d'erreur sans %q : %s", want, msg)
		}
	}
}

// TestDiffMoves vérifie la détection des fichiers déplacés (single → album) :
// même identité (URL Spotify), rel_path différent → un FileMove est émis.
// Une suppression seule ne doit PAS être traitée comme un déplacement.
func TestDiffMoves(t *testing.T) {
	before := map[string][]string{
		"url:https://open.spotify.com/track/aaa": {"Ninho/Vrais/Vrais.mp3.mp3"},
		"url:https://open.spotify.com/track/bbb": {"Jul/Zone/Zone.mp3.mp3"},
		"url:https://open.spotify.com/track/ccc": {"Jul/Album/Delete.mp3.mp3"}, // supprimé → ignoré
	}
	after := map[string][]string{
		"url:https://open.spotify.com/track/aaa": {"Ninho/M.I.L.S 2.0/Vrais.mp3.mp3"},
		"url:https://open.spotify.com/track/bbb": {"Jul/Zone/Zone.mp3.mp3"},
	}

	moves := diffMoves(before, after)
	if len(moves) != 1 {
		t.Fatalf("want 1 move, got %d: %+v", len(moves), moves)
	}
	if moves[0].OldRel != "Ninho/Vrais/Vrais.mp3.mp3" {
		t.Errorf("unexpected old path: %q", moves[0].OldRel)
	}
	if moves[0].NewRel != "Ninho/M.I.L.S 2.0/Vrais.mp3.mp3" {
		t.Errorf("unexpected new path: %q", moves[0].NewRel)
	}

	// Aucun changement → aucun move.
	if got := diffMoves(after, after); len(got) != 0 {
		t.Fatalf("want 0 moves for identical maps, got %+v", got)
	}

	// Cas multi-copies (même URL sur plusieurs fichiers) → on ne migre pas
	// au hasard.
	multi := map[string][]string{
		"url:https://open.spotify.com/track/aaa": {"A/1.mp3", "B/2.mp3"},
	}
	afterOne := map[string][]string{
		"url:https://open.spotify.com/track/aaa": {"B/2.mp3"},
	}
	if got := diffMoves(multi, afterOne); len(got) != 0 {
		t.Fatalf("want 0 moves for multi-copy, got %+v", got)
	}
}

// TestQueuePersistence vérifie qu'une tâche en vol survit à un redémarrage :
// persist() l'écrit, recoverQueue() la re-file avec le statut "queued".
func TestQueuePersistence(t *testing.T) {
	dir := t.TempDir()
	persist := filepath.Join(dir, "queue.json")

	newMgr := func() *Manager {
		return &Manager{
			tasks:       make(map[string]*DownloadTask),
			queue:       make(chan *DownloadTask, 100),
			persistPath: persist,
		}
	}

	m := newMgr()
	m.tasks["task_1"] = &DownloadTask{
		ID:        "task_1",
		URL:       "https://example.com/playlist/abc",
		Bitrate:   "320k",
		Order:     "reverse",
		Status:    StatusQueued,
		Progress:  "In queue...",
		CreatedAt: time.Now(),
	}
	// Une tâche terminée ne doit PAS être re-filée après restart.
	m.tasks["task_2"] = &DownloadTask{
		ID:        "task_2",
		URL:       "https://example.com/track/xyz",
		Bitrate:   "320k",
		Status:    StatusCompleted,
		CreatedAt: time.Now(),
	}

	m.persist()

	// Simule un redémarrage : nouveau Manager, même fichier.
	m2 := newMgr()
	m2.recoverQueue()

	if len(m2.tasks) != 1 {
		t.Fatalf("expected 1 recovered task, got %d", len(m2.tasks))
	}
	task, ok := m2.tasks["task_1"]
	if !ok {
		t.Fatal("task_1 not recovered")
	}
	if task.Status != StatusQueued {
		t.Fatalf("expected recovered task to be queued, got %s", task.Status)
	}
	if task.Progress != "Re-queued after server restart" {
		t.Fatalf("unexpected progress: %q", task.Progress)
	}
	if len(m2.queue) != 1 {
		t.Fatalf("expected 1 task in queue, got %d", len(m2.queue))
	}
}

// TestRecoverQueueCorruptFile vérifie qu'un fichier corrompu ne fait pas
// planter le démarrage (il est simplement ignoré).
func TestRecoverQueueCorruptFile(t *testing.T) {
	dir := t.TempDir()
	persist := filepath.Join(dir, "queue.json")
	if err := os.WriteFile(persist, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := &Manager{
		tasks:       make(map[string]*DownloadTask),
		queue:       make(chan *DownloadTask, 100),
		persistPath: persist,
	}
	m.recoverQueue() // ne doit pas paniquer
	if len(m.tasks) != 0 {
		t.Fatalf("expected no tasks, got %d", len(m.tasks))
	}
}
