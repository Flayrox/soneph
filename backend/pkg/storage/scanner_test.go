package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListFiles(t *testing.T) {
	dir := t.TempDir()

	// Structure type spotdl : {artist}/{album}/{title}.mp3
	writeFile(t, filepath.Join(dir, "Ninho", "Jefe", "Putana.mp3"), "audio1")
	writeFile(t, filepath.Join(dir, "Ninho", "Jefe", "Putana.lrc"), "[00:12.34]Paroles synchronisées\n")
	writeFile(t, filepath.Join(dir, "Ninho", "Jefe", "La puerta.mp3"), "audio2")
	writeFile(t, filepath.Join(dir, "Ninho", "Jefe", "La puerta.lrc"), "Paroles en texte brut\n")
	// Pas de .lrc
	writeFile(t, filepath.Join(dir, "Jul", "Album2", "Tchikita.mp3"), "audio3")
	// Fichier ignoré (pas audio)
	writeFile(t, filepath.Join(dir, "Jul", "Album2", "cover.jpg"), "img")

	s := NewScanner(dir)
	files, err := s.ListFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 audio files, got %d", len(files))
	}

	byTitle := map[string]DownloadedFile{}
	for _, f := range files {
		byTitle[f.Title] = f
	}

	putana := byTitle["Putana"]
	if putana.Artist != "Ninho" || putana.Album != "Jefe" {
		t.Fatalf("wrong artist/album: %q / %q", putana.Artist, putana.Album)
	}
	if !putana.HasLyrics || putana.LyricsType != "synced" {
		t.Fatalf("Putana should be synced: %v / %q", putana.HasLyrics, putana.LyricsType)
	}

	puerta := byTitle["La puerta"]
	if !puerta.HasLyrics || puerta.LyricsType != "unsynced" {
		t.Fatalf("La puerta should be unsynced: %v / %q", puerta.HasLyrics, puerta.LyricsType)
	}

	tchikita := byTitle["Tchikita"]
	if tchikita.HasLyrics || tchikita.LyricsType != "none" {
		t.Fatalf("Tchikita should have no lyrics: %v / %q", tchikita.HasLyrics, tchikita.LyricsType)
	}
}

func TestListFilesFlatLayout(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Solo Track.mp3"), "audio")

	s := NewScanner(dir)
	files, err := s.ListFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Artist != "Unknown Artist" {
		t.Fatalf("expected Unknown Artist, got %q", files[0].Artist)
	}
}

func TestDeleteFileRemovesLrc(t *testing.T) {
	dir := t.TempDir()
	audio := filepath.Join(dir, "Art", "Al", "Song.mp3")
	lrc := filepath.Join(dir, "Art", "Al", "Song.lrc")
	writeFile(t, audio, "a")
	writeFile(t, lrc, "l")

	s := NewScanner(dir)
	if err := s.DeleteFile("Art/Al/Song.mp3"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(audio); !os.IsNotExist(err) {
		t.Fatal("audio file should be deleted")
	}
	if _, err := os.Stat(lrc); !os.IsNotExist(err) {
		t.Fatal("lrc file should be deleted too")
	}
}

// TestResolvePathRelativeDir : un base dir relatif (ex. "./downloads") ne doit
// pas casser la résolution — le bug qui rejetait tous les chemins.
func TestResolvePathRelativeDir(t *testing.T) {
	s := NewScanner("./downloads")

	full, err := s.ResolvePath("Art/Al/Song.mp3")
	if err != nil {
		t.Fatalf("relative base dir should resolve fine, got: %v", err)
	}
	if full != filepath.Join("downloads", "Art", "Al", "Song.mp3") {
		t.Fatalf("unexpected resolved path: %q", full)
	}

	// La traversée reste refusée même avec un base relatif.
	if _, err := s.ResolvePath("../outside.mp3"); err == nil {
		t.Fatal("expected traversal to be rejected with a relative base")
	}
}

// TestDeleteFilePathTraversal vérifie qu'un rel_path avec ../ ne sort pas du
// dossier de téléchargements.
func TestDeleteFilePathTraversal(t *testing.T) {
	dir := t.TempDir()
	s := NewScanner(dir)

	// Écrit un fichier hors du dossier de téléchargements.
	outside := filepath.Join(t.TempDir(), "victim.txt")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := s.DeleteFile("../../../../" + filepath.Base(outside))
	if err == nil {
		t.Fatal("expected a path traversal error")
	}
	if _, statErr := os.Stat(outside); statErr != nil {
		t.Fatal("file outside download dir must not be deleted")
	}
}
