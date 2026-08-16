// Package history records play events (scrobbles) so the UI can show
// recently played tracks and per-track play counts. Stored as a capped JSON
// file in the app config dir.
package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type Record struct {
	Path     string    `json:"path"`
	PlayedAt time.Time `json:"played_at"`
}

type Store struct {
	mu   sync.Mutex
	path string
	max  int
}

const DefaultMax = 500

func path() string {
	if p := os.Getenv("SONEPH_HISTORY_FILE"); p != "" {
		return p
	}
	d, err := os.UserConfigDir()
	if err != nil {
		d = os.TempDir()
	}
	return filepath.Join(d, "soneph", "history.json")
}

func New() *Store {
	return &Store{path: path(), max: DefaultMax}
}

func (s *Store) load() []Record {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil
	}
	var recs []Record
	if err := json.Unmarshal(data, &recs); err != nil {
		return nil
	}
	return recs
}

func (s *Store) save(recs []Record) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

// Add records a play, newest first, capped at max entries.
func (s *Store) Add(track string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	recs := s.load()
	// A repeated play of the same track while it's already playing would
	// pollute the history — ignore back-to-back duplicates of the newest one.
	if len(recs) > 0 && recs[0].Path == track {
		recs[0].PlayedAt = time.Now()
	} else {
		recs = append([]Record{{Path: track, PlayedAt: time.Now()}}, recs...)
	}
	if len(recs) > s.max {
		recs = recs[:s.max]
	}
	_ = s.save(recs)
}

// Recent returns the last n records (oldest-last order is kept).
func (s *Store) Recent(limit int) []Record {
	s.mu.Lock()
	defer s.mu.Unlock()

	recs := s.load()
	if limit > 0 && len(recs) > limit {
		recs = recs[:limit]
	}
	return recs
}

// MostPlayed returns distinct tracks sorted by play count, descending.
type Count struct {
	Path  string `json:"path"`
	Plays int    `json:"plays"`
}

func (s *Store) MostPlayed(limit int) []Count {
	s.mu.Lock()
	defer s.mu.Unlock()

	counts := map[string]int{}
	for _, r := range s.load() {
		counts[r.Path]++
	}
	out := make([]Count, 0, len(counts))
	for p, c := range counts {
		out = append(out, Count{Path: p, Plays: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Plays == out[j].Plays {
			return out[i].Path < out[j].Path
		}
		return out[i].Plays > out[j].Plays
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (s *Store) Total() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.load())
}
