package downloader

import (
	"os"
	"path/filepath"
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
