// Package history records play events (scrobbles) so the UI can show
// recently played tracks and per-track play counts. Stored as a capped JSON
// file in the app config dir.
package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Record struct {
	Path     string    `json:"path"`
	Duration int       `json:"duration"` // seconds, 0 if unknown
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
func (s *Store) Add(track string, duration int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	recs := s.load()
	// A repeated play of the same track while it's already playing would
	// pollute the history — ignore back-to-back duplicates of the newest one.
	if len(recs) > 0 && recs[0].Path == track {
		recs[0].PlayedAt = time.Now()
		recs[0].Duration = duration
	} else {
		recs = append([]Record{{Path: track, Duration: duration, PlayedAt: time.Now()}}, recs...)
	}
	if len(recs) > s.max {
		recs = recs[:s.max]
	}
	_ = s.save(recs)
}

// Rename migre l'historique d'un ancien chemin vers un nouveau (un morceau
// déplacé, ex. single → album, garde ses écoutes).
func (s *Store) Rename(oldPath, newPath string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	recs := s.load()
	changed := false
	for i := range recs {
		if recs[i].Path == oldPath {
			recs[i].Path = newPath
			changed = true
		}
	}
	if changed {
		_ = s.save(recs)
	}
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
	return s.mostPlayedLocked(limit)
}

func (s *Store) mostPlayedLocked(limit int) []Count {
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

// Stats aggregates the whole history: total plays, listening time, top
// artists (derived from the artist/album/title folder layout) and plays per
// day for the last two weeks.
type Stats struct {
	TotalPlays    int              `json:"total_plays"`
	TotalSeconds  int              `json:"total_seconds"`
	TopArtists    []ArtistCount    `json:"top_artists"`
	TopTracks     []Count          `json:"top_tracks"`
	PlaysByDay    []DayCount       `json:"plays_by_day"`
}

type ArtistCount struct {
	Artist string `json:"artist"`
	Plays  int    `json:"plays"`
}

type DayCount struct {
	Day   string `json:"day"` // YYYY-MM-DD
	Plays int    `json:"plays"`
}

func (s *Store) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()

	recs := s.load()

	st := Stats{TotalPlays: len(recs)}
	artists := map[string]int{}
	days := map[string]int{}
	now := time.Now()

	for _, r := range recs {
		st.TotalSeconds += r.Duration

		// artist = first path segment of the {artist}/{album}/{title} layout.
		if i := strings.IndexByte(r.Path, os.PathSeparator); i > 0 {
			artists[r.Path[:i]]++
		}

		// Keep only the last 14 days (including today) in the daily chart.
		day := r.PlayedAt.Format("2006-01-02")
		if diff := now.Sub(r.PlayedAt); diff >= 0 && diff <= 14*24*time.Hour {
			days[day]++
		}
	}

	for a, c := range artists {
		st.TopArtists = append(st.TopArtists, ArtistCount{Artist: a, Plays: c})
	}
	sort.Slice(st.TopArtists, func(i, j int) bool {
		if st.TopArtists[i].Plays == st.TopArtists[j].Plays {
			return st.TopArtists[i].Artist < st.TopArtists[j].Artist
		}
		return st.TopArtists[i].Plays > st.TopArtists[j].Plays
	})
	if len(st.TopArtists) > 10 {
		st.TopArtists = st.TopArtists[:10]
	}

	st.TopTracks = s.mostPlayedLocked(10)

	// Fill all 14 days so the chart is continuous (zero for quiet days).
	for i := 13; i >= 0; i-- {
		d := now.AddDate(0, 0, -i).Format("2006-01-02")
		st.PlaysByDay = append(st.PlaysByDay, DayCount{Day: d, Plays: days[d]})
	}

	return st
}
