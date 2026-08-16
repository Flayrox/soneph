package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// LikesStore keeps the set of liked tracks (by rel_path). Stored as a JSON
// file in the app config dir so likes survive restarts.
type LikesStore struct {
	mu   sync.Mutex
	path string
}

func likesPath() string {
	if p := os.Getenv("SONEPH_LIKES_FILE"); p != "" {
		return p
	}
	d, err := os.UserConfigDir()
	if err != nil {
		d = os.TempDir()
	}
	return filepath.Join(d, "soneph", "likes.json")
}

func NewLikes() *LikesStore {
	return &LikesStore{path: likesPath()}
}

func (s *LikesStore) load() map[string]bool {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return map[string]bool{}
	}
	var m map[string]bool
	if err := json.Unmarshal(data, &m); err != nil {
		return map[string]bool{}
	}
	return m
}

func (s *LikesStore) save(m map[string]bool) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

// Add marks a track as liked. Returns true if it was newly liked.
func (s *LikesStore) Add(track string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.load()
	added := !m[track]
	m[track] = true
	return added, s.save(m)
}

// Remove unlikes a track. Returns true if it was previously liked.
func (s *LikesStore) Remove(track string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.load()
	removed := m[track]
	delete(m, track)
	return removed, s.save(m)
}

// Rename migre un like d'un ancien chemin vers un nouveau (un morceau
// déplacé, ex. single → album, garde son cœur).
func (s *LikesStore) Rename(oldPath, newPath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.load()
	if m[oldPath] {
		delete(m, oldPath)
		m[newPath] = true
		_ = s.save(m)
	}
}

// List returns the sorted list of liked rel_paths.
func (s *LikesStore) List() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.load()
	out := make([]string, 0, len(m))
	for p := range m {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
