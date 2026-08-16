// Package playlists stores user playlists as small JSON files. A playlist is
// a named, ordered list of rel_paths (relative to the downloads directory) —
// the same paths the scanner reports, so resolving a playlist is a simple
// lookup against the scanned library.
package playlists

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Playlist struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Tracks    []string  `json:"tracks"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Summary is the lightweight shape used in the playlist list.
type Summary struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	TrackCount int       `json:"track_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

var ErrNotFound = errors.New("playlist not found")

type Store struct {
	mu  sync.Mutex
	dir string
}

func dir() string {
	if p := os.Getenv("SONEPH_PLAYLISTS_DIR"); p != "" {
		return p
	}
	d, err := os.UserConfigDir()
	if err != nil {
		d = os.TempDir()
	}
	return filepath.Join(d, "soneph", "playlists")
}

func New() *Store {
	return &Store{dir: dir()}
}

func newID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "pl_" + hex.EncodeToString(b)
}

func (s *Store) path(id string) string {
	return filepath.Join(s.dir, id+".json")
}

func (s *Store) load(id string) (Playlist, error) {
	data, err := os.ReadFile(s.path(id))
	if err != nil {
		return Playlist{}, ErrNotFound
	}
	var p Playlist
	if err := json.Unmarshal(data, &p); err != nil {
		return Playlist{}, ErrNotFound
	}
	return p, nil
}

func (s *Store) save(p Playlist) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(p.ID), data, 0o644)
}

// List returns every playlist, most recently updated first.
func (s *Store) List() ([]Summary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Summary{}, nil
		}
		return nil, err
	}

	out := []Summary{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		p, err := s.load(id)
		if err != nil {
			continue
		}
		out = append(out, Summary{
			ID:         p.ID,
			Name:       p.Name,
			TrackCount: len(p.Tracks),
			CreatedAt:  p.CreatedAt,
			UpdatedAt:  p.UpdatedAt,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}

func (s *Store) Create(name string) (Playlist, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	name = strings.TrimSpace(name)
	if name == "" {
		name = "Playlist"
	}
	p := Playlist{
		ID:        newID(),
		Name:      name,
		Tracks:    []string{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.save(p); err != nil {
		return Playlist{}, err
	}
	return p, nil
}

func (s *Store) Get(id string) (Playlist, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load(id)
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.load(id); err != nil {
		return err
	}
	return os.Remove(s.path(id))
}

// AddTrack appends a track unless it's already present (no duplicates).
func (s *Store) AddTrack(id, track string) (Playlist, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, err := s.load(id)
	if err != nil {
		return Playlist{}, err
	}
	for _, t := range p.Tracks {
		if t == track {
			return p, nil
		}
	}
	p.Tracks = append(p.Tracks, track)
	p.UpdatedAt = time.Now()
	if err := s.save(p); err != nil {
		return Playlist{}, err
	}
	return p, nil
}

func (s *Store) RemoveTrack(id, track string) (Playlist, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, err := s.load(id)
	if err != nil {
		return Playlist{}, err
	}
	out := p.Tracks[:0]
	for _, t := range p.Tracks {
		if t != track {
			out = append(out, t)
		}
	}
	p.Tracks = out
	p.UpdatedAt = time.Now()
	if err := s.save(p); err != nil {
		return Playlist{}, err
	}
	return p, nil
}

// Reorder replaces the playlist's track order with the given list (used by
// drag-and-drop reordering). Unknown paths are dropped; the rest keep the
// provided order.
func (s *Store) Reorder(id string, tracks []string) (Playlist, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, err := s.load(id)
	if err != nil {
		return Playlist{}, err
	}
	known := make(map[string]bool, len(p.Tracks))
	for _, t := range p.Tracks {
		known[t] = true
	}
	out := make([]string, 0, len(tracks))
	seen := make(map[string]bool, len(tracks))
	for _, t := range tracks {
		if !known[t] || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	// Keep any tracks the client didn't mention (append in original order).
	for _, t := range p.Tracks {
		if !seen[t] {
			out = append(out, t)
		}
	}
	p.Tracks = out
	p.UpdatedAt = time.Now()
	if err := s.save(p); err != nil {
		return Playlist{}, err
	}
	return p, nil
}
