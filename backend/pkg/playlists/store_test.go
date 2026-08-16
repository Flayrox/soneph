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

// TestRenameTrack: un morceau déplacé (single → album) garde sa place dans
// les playlists sous son nouveau chemin.
func TestRenameTrack(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.Create("Chill")
	_, _ = s.AddTrack(p.ID, "Ninho/Vrais/Vrais.mp3.mp3")
	_, _ = s.AddTrack(p.ID, "Jul/Zone/Zone.mp3.mp3")

	s.RenameTrack("Ninho/Vrais/Vrais.mp3.mp3", "Ninho/M.I.L.S 2.0/Vrais.mp3.mp3")

	p, _ = s.Get(p.ID)
	if len(p.Tracks) != 2 {
		t.Fatalf("want 2 tracks, got %+v", p.Tracks)
	}
	if p.Tracks[0] != "Ninho/M.I.L.S 2.0/Vrais.mp3.mp3" {
		t.Errorf("want renamed track first, got %+v", p.Tracks)
	}

	// Si le nouveau chemin existe déjà, pas de doublon : on retire l'ancien.
	p2, _ := s.Create("NoDup")
	_, _ = s.AddTrack(p2.ID, "Art/Al/old.mp3")
	_, _ = s.AddTrack(p2.ID, "Art/Al/new.mp3")
	s.RenameTrack("Art/Al/old.mp3", "Art/Al/new.mp3")
	p2, _ = s.Get(p2.ID)
	if len(p2.Tracks) != 1 || p2.Tracks[0] != "Art/Al/new.mp3" {
		t.Fatalf("want single new track, got %+v", p2.Tracks)
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
