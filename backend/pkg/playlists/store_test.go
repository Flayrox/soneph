package playlists

import (
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	t.Setenv("SONEPH_PLAYLISTS_DIR", filepath.Join(t.TempDir(), "playlists"))
	return New()
}

func TestCreateAndGet(t *testing.T) {
	s := newTestStore(t)
	p, err := s.Create("Chill")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Chill" || len(p.Tracks) != 0 || p.ID == "" {
		t.Fatalf("unexpected playlist: %+v", p)
	}

	got, err := s.Get(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Chill" {
		t.Fatalf("expected Chill, got %q", got.Name)
	}
}

func TestAddRemoveTrackNoDuplicates(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.Create("T")

	if _, err := s.AddTrack(p.ID, "Art/Al/1.mp3"); err != nil {
		t.Fatal(err)
	}
	// Même morceau deux fois → pas de doublon
	if _, err := s.AddTrack(p.ID, "Art/Al/1.mp3"); err != nil {
		t.Fatal(err)
	}
	p, _ = s.Get(p.ID)
	if len(p.Tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(p.Tracks))
	}

	if _, err := s.RemoveTrack(p.ID, "Art/Al/1.mp3"); err != nil {
		t.Fatal(err)
	}
	p, _ = s.Get(p.ID)
	if len(p.Tracks) != 0 {
		t.Fatalf("expected 0 tracks, got %d", len(p.Tracks))
	}
}

func TestDelete(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.Create("X")
	if err := s.Delete(p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(p.ID); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	// Supprimer une playlist inexistante → erreur
	if err := s.Delete("pl_nope"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound on delete, got %v", err)
	}
}

func TestPersistenceAcrossInstances(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.Create("Persisted")
	_, _ = s.AddTrack(p.ID, "Art/Al/song.mp3")

	// Nouvelle instance, même dossier → les données survivent.
	s2 := New()
	list, err := s2.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "Persisted" || list[0].TrackCount != 1 {
		t.Fatalf("unexpected list after reload: %+v", list)
	}
}

func TestEmptyNameFallsBack(t *testing.T) {
	s := newTestStore(t)
	p, err := s.Create("   ")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Playlist" {
		t.Fatalf("expected fallback name, got %q", p.Name)
	}
}
