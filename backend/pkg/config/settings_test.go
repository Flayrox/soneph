package config

import (
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	t.Setenv("SONEPH_CONFIG", filepath.Join(t.TempDir(), "nonexistent.json"))
	s := Load()
	if s.Workers != DefaultWorkers {
		t.Fatalf("expected %d workers, got %d", DefaultWorkers, s.Workers)
	}
	if s.Threads != DefaultThreads {
		t.Fatalf("expected %d threads, got %d", DefaultThreads, s.Threads)
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "soneph", "settings.json")
	t.Setenv("SONEPH_CONFIG", path)

	want := Settings{Workers: 2, Threads: 8}
	if err := Save(want); err != nil {
		t.Fatal(err)
	}
	got := Load()
	if got != want {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
}

func TestInvalidValuesClampedToDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "soneph", "settings.json")
	t.Setenv("SONEPH_CONFIG", path)

	if err := Save(Settings{Workers: -5, Threads: 0}); err != nil {
		t.Fatal(err)
	}
	got := Load()
	if got.Workers != DefaultWorkers || got.Threads != DefaultThreads {
		t.Fatalf("expected defaults, got %+v", got)
	}
}
