package history

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("SONEPH_HISTORY_FILE", filepath.Join(dir, "history.json"))
	return New()
}

func TestAddAndRecent(t *testing.T) {
	s := newTestStore(t)
	s.Add("artist/album/track1.mp3")
	s.Add("artist/album/track2.mp3")

	recs := s.Recent(10)
	if len(recs) != 2 {
		t.Fatalf("want 2 records, got %d", len(recs))
	}
	// Newest first.
	if recs[0].Path != "artist/album/track2.mp3" {
		t.Errorf("want newest first, got %q", recs[0].Path)
	}
}

func TestBackToBackDuplicateIsUpdated(t *testing.T) {
	s := newTestStore(t)
	s.Add("a.mp3")
	s.Add("a.mp3") // replaying the newest track must not duplicate it

	recs := s.Recent(10)
	if len(recs) != 1 {
		t.Fatalf("want 1 record after replay, got %d", len(recs))
	}
}

func TestRecentLimit(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 10; i++ {
		s.Add("track")
	}
	// All the same track -> capped at 1 due to dedupe.
	if got := len(s.Recent(5)); got != 1 {
		t.Fatalf("want 1 record, got %d", got)
	}
}

func TestMostPlayed(t *testing.T) {
	s := newTestStore(t)
	s.Add("a.mp3")
	s.Add("b.mp3")
	s.Add("a.mp3") // replaying after another track counts as a new play

	top := s.MostPlayed(10)
	if len(top) != 2 {
		t.Fatalf("want 2 distinct tracks, got %d", len(top))
	}
	if top[0].Path != "a.mp3" || top[0].Plays != 2 {
		t.Errorf("want a.mp3 x2 first, got %+v", top[0])
	}
}

func TestMaxCap(t *testing.T) {
	s := newTestStore(t)
	// Distinct paths so nothing gets deduped.
	for i := 0; i < DefaultMax+50; i++ {
		s.Add("track_" + string(rune('a'+i%26)) + "_" + string(rune('0'+i%10)) + ".mp3")
	}
	if got := len(s.Recent(0)); got > DefaultMax {
		t.Fatalf("history should be capped at %d, got %d", DefaultMax, got)
	}
}

func TestLikesAddRemoveList(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SONEPH_LIKES_FILE", filepath.Join(dir, "likes.json"))
	ls := NewLikes()

	added, err := ls.Add("a.mp3")
	if err != nil || !added {
		t.Fatalf("add: err=%v added=%v", err, added)
	}
	// Adding again is idempotent.
	added2, _ := ls.Add("a.mp3")
	if added2 {
		t.Error("second add should report not-added")
	}
	if _, err := ls.Add("b.mp3"); err != nil {
		t.Fatal(err)
	}
	got := ls.List()
	if len(got) != 2 || got[0] != "a.mp3" || got[1] != "b.mp3" {
		t.Errorf("unexpected list: %v", got)
	}

	removed, err := ls.Remove("a.mp3")
	if err != nil || !removed {
		t.Fatalf("remove: err=%v removed=%v", err, removed)
	}
	got = ls.List()
	if len(got) != 1 || got[0] != "b.mp3" {
		t.Errorf("unexpected list after remove: %v", got)
	}
}

// TestPersistenceAcrossInstances: likes + history survive a restart (new instance).
func TestPersistenceAcrossInstances(t *testing.T) {
	dir := t.TempDir()
	histFile := filepath.Join(dir, "history.json")
	likesFile := filepath.Join(dir, "likes.json")
	t.Setenv("SONEPH_HISTORY_FILE", histFile)
	t.Setenv("SONEPH_LIKES_FILE", likesFile)

	s1 := New()
	s1.Add("persist.mp3")
	ls1 := NewLikes()
	_, _ = ls1.Add("persist.mp3")

	// New instances, same files.
	s2 := New()
	if recs := s2.Recent(10); len(recs) != 1 || recs[0].Path != "persist.mp3" {
		t.Errorf("history did not persist: %v", recs)
	}
	ls2 := NewLikes()
	if got := ls2.List(); len(got) != 1 || got[0] != "persist.mp3" {
		t.Errorf("likes did not persist: %v", got)
	}

	_ = os.Remove(histFile)
}
