package fastfilter

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// fixtureFetch renvoie la fixture demandée pour toute URL — l'équivalent du
// fetch injectable des tests de pagination.
func fixtureFetch(name string) FetchFunc {
	html, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		panic(err)
	}
	return func(string) ([]byte, error) { return html, nil }
}

// TestResolvePlaylist vérifie la résolution par IDENTITÉ (URL Spotify dans
// les tags WOAS, via la carte passée en argument) : les morceaux déjà sur
// disque sont matched avec leur rel_path, les autres missing — comme
// playlist_from_url.py.
func TestResolvePlaylist(t *testing.T) {
	identity := map[string][]string{
		"url:https://open.spotify.com/track/aaa": {"Ninho/Vrais.mp3.mp3"},
	}
	res, err := ResolvePlaylist("https://open.spotify.com/playlist/abc123", fixtureFetch("playlist.html"), identity)
	if err != nil {
		t.Fatalf("ResolvePlaylist: %v", err)
	}
	if res.Name != "Vrais — Playlist de test" {
		t.Errorf("Name = %q, want le nom de la fixture", res.Name)
	}
	if res.Total != 3 || res.Truncated {
		t.Errorf("Total = %d, Truncated = %v; want 3, false", res.Total, res.Truncated)
	}
	wantMatched := []PlaylistTrack{{Title: "Vrais", Artist: "Ninho", URI: "spotify:track:aaa", RelPath: "Ninho/Vrais.mp3.mp3"}}
	if !reflect.DeepEqual(res.Matched, wantMatched) {
		t.Errorf("Matched = %+v, want %+v", res.Matched, wantMatched)
	}
	if len(res.Missing) != 2 {
		t.Fatalf("Missing = %d morceaux, want 2", len(res.Missing))
	}
	if res.Missing[0].URI != "spotify:track:bbb" || res.Missing[0].RelPath != "" {
		t.Errorf("Missing[0] = %+v, want bbb sans rel_path", res.Missing[0])
	}
	// Morceau sans uri (pas de spotify:track:) → missing, sans lookup.
	if res.Missing[1].URI != "spotify:track:ccc" {
		t.Errorf("Missing[1] = %+v, want ccc", res.Missing[1])
	}
}

// TestResolvePlaylistNoMatch : aucune identité ne correspond → tout est
// missing, pas d'erreur.
func TestResolvePlaylistNoMatch(t *testing.T) {
	res, err := ResolvePlaylist("https://open.spotify.com/playlist/abc123", fixtureFetch("playlist.html"), nil)
	if err != nil {
		t.Fatalf("ResolvePlaylist: %v", err)
	}
	if len(res.Matched) != 0 || len(res.Missing) != 3 {
		t.Errorf("Matched=%d Missing=%d, want 0 et 3", len(res.Matched), len(res.Missing))
	}
}

// TestResolvePlaylistTrack : un morceau seul (page sans trackList) est
// résolu via l'entité ; le nom retombe sur le titre.
func TestResolvePlaylistTrack(t *testing.T) {
	res, err := ResolvePlaylist("https://open.spotify.com/track/xyz", fixtureFetch("track.html"), nil)
	if err != nil {
		t.Fatalf("ResolvePlaylist: %v", err)
	}
	if res.Name != "La puerta" {
		t.Errorf("Name = %q, want La puerta (title de l'entité)", res.Name)
	}
	if res.Total != 1 || len(res.Missing) != 1 || res.Missing[0].URI != "spotify:track:xyz" {
		t.Errorf("résultat = %+v, want 1 morceau missing (xyz)", res)
	}
}

// TestResolvePlaylistInvalidURL : lien sans playlist/album/track → erreur
// (le handler n'a alors pas de quoi créer la playlist).
func TestResolvePlaylistInvalidURL(t *testing.T) {
	if _, err := ResolvePlaylist("https://example.com/not-a-media-link", nil, nil); err == nil {
		t.Error("lien invalide : aucune erreur renvoyée")
	}
}
