package fastfilter

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── Normalize ────────────────────────────────────────────────────────────

func TestNormalize(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Vrais", "vrais"},
		{"Goutte d'eau", "goutte deau"},
		{"Peur d'aimer (Remastered)", "peur daimer"},
		{"Zone [Bonus Track]", "zone"},
		{"Karma {Version 2}", "karma"},
		{"  Par Amour  ", "par amour"},
		// Le tiret disparaît et laisse un double espace (comportement
		// Python : re.sub ne condense pas les espaces) — les deux côtés
		// (fichier et requête) subissent la même normalisation.
		{"Ninho - Vrais", "ninho  vrais"},
		{"L'Été", "lété"},
		{"123 Soleil", "123 soleil"},
	}
	for _, c := range cases {
		if got := Normalize(c.in); got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ── Parse des fixtures ───────────────────────────────────────────────────

func TestParseTrackListFixture(t *testing.T) {
	html, err := os.ReadFile(filepath.Join("testdata", "playlist.html"))
	if err != nil {
		t.Fatal(err)
	}
	tracks := parseTrackList(html)
	if len(tracks) != 3 {
		t.Fatalf("parseTrackList = %d morceaux, want 3", len(tracks))
	}
	if tracks[0].Title != "Vrais" || tracks[0].Artist != "Ninho" || tracks[0].Query != "Ninho - Vrais" {
		t.Errorf("morceau 0 = %+v", tracks[0])
	}
	// Subtitle vide → le champ artists de secours n'est pas non plus rempli
	// dans la fixture : query = titre seul.
	if tracks[2].Query != "Peur d'aimer" {
		t.Errorf("query du morceau sans artiste = %q, want titre seul", tracks[2].Query)
	}
}

func TestParseTrackEntityFixture(t *testing.T) {
	html, err := os.ReadFile(filepath.Join("testdata", "track.html"))
	if err != nil {
		t.Fatal(err)
	}
	tracks := parseTrackEntity(html)
	if len(tracks) != 1 {
		t.Fatalf("parseTrackEntity = %d morceaux, want 1", len(tracks))
	}
	tr := tracks[0]
	if tr.Title != "La puerta" || tr.Artist != "Jul, Naps" || tr.Query != "Jul, Naps - La puerta" {
		t.Errorf("track = %+v", tr)
	}
}

// ── ExistingSet (lookup O(N)) ────────────────────────────────────────────

func TestExistingSet(t *testing.T) {
	dir := t.TempDir()
	// spotdl écrit « Title.mp3.mp3 » : jusqu'à 2 extensions retirées.
	files := map[string]string{
		"Ninho/Vrais.mp3":                "vrais",
		"Jul/Zone.mp3.mp3":               "zone",
		"Naps/Par Amour (Bonus).mp3.mp3": "par amour",
		"playlist.m3u8":                  "", // ignoré : pas .mp3
		"cover.jpg":                      "", // ignoré
	}
	for rel, _ := range files {
		_ = os.MkdirAll(filepath.Dir(filepath.Join(dir, rel)), 0o755)
		if rel != "" {
			_ = os.WriteFile(filepath.Join(dir, rel), []byte("x"), 0o644)
		}
	}

	set, err := ExistingSet(dir)
	if err != nil {
		t.Fatal(err)
	}
	for rel, want := range files {
		if want == "" {
			continue
		}
		if _, ok := set[want]; !ok {
			t.Errorf("set sans %q (fichier %s)", want, rel)
		}
	}
	if len(set) != 3 {
		t.Errorf("set = %d entrées, want 3 (les non-.mp3 sont ignorés)", len(set))
	}

	// Dossier inexistant → set vide, pas d'erreur (parité glob.glob).
	empty, err := ExistingSet(filepath.Join(dir, "nope"))
	if err != nil || len(empty) != 0 {
		t.Errorf("ExistingSet(dossier absent) = %d, %v; want 0, nil", len(empty), err)
	}
}

// ── Pagination (pages répétées, MAX_PAGES, page courte) ──────────────────

// pageFixtureFrom génère une page embed avec n morceaux dont les titres
// démarrent à l'index off (les pages sont donc distinctes, contrairement à
// une vraie réponse d'API qui répète la page quand elle ignore l'offset).
func pageFixtureFrom(off, n int) []byte {
	trackList := make([]map[string]string, 0, n)
	for i := 0; i < n; i++ {
		k := off + i
		trackList = append(trackList, map[string]string{
			"title":    fmt.Sprintf("Song %03d", k),
			"subtitle": fmt.Sprintf("Artist %03d", k),
			"uri":      fmt.Sprintf("spotify:track:%03d", k),
		})
	}
	data := map[string]any{
		"props": map[string]any{
			"pageProps": map[string]any{
				"state": map[string]any{
					"data": map[string]any{
						"entity": map[string]any{"trackList": trackList},
					},
				},
			},
		},
	}
	raw, _ := json.Marshal(data)
	return []byte(`<script id="__NEXT_DATA__" type="application/json">` + string(raw) + `</script>`)
}

func pageFixture(n int) []byte { return pageFixtureFrom(0, n) }

// fakePages construit un fetch qui renvoie des pages distinctes de perPage
// morceaux jusqu'à la page maxPage, puis une page vide (fin de liste).
func fakePages(perPage, maxPage int) FetchFunc {
	return func(rawURL string) ([]byte, error) {
		u, _ := url.Parse(rawURL)
		off := 0
		if v := u.Query().Get("offset"); v != "" {
			fmt.Sscanf(v, "%d", &off)
		}
		page := off/PageSize + 1
		if page > maxPage {
			return nil, nil // page vide → fin de liste
		}
		return pageFixtureFrom(off, perPage), nil
	}
}

func TestFetchAllTracksPlaylistShortPage(t *testing.T) {
	// 3 pages : 100 + 100 + 50 — la page courte marque la fin.
	fetch := func(rawURL string) ([]byte, error) {
		u, _ := url.Parse(rawURL)
		off := 0
		fmt.Sscanf(u.Query().Get("offset"), "%d", &off)
		if off < 2*PageSize {
			return pageFixtureFrom(off, PageSize), nil
		}
		if off == 2*PageSize {
			return pageFixtureFrom(off, PageSize/2), nil
		}
		return nil, nil
	}
	tracks, truncated, err := FetchAllTracks("https://open.spotify.com/playlist/abc123", fetch)
	if err != nil {
		t.Fatal(err)
	}
	if truncated {
		t.Error("truncated = true, want false (page courte = fin de liste)")
	}
	if len(tracks) != PageSize+PageSize+PageSize/2 {
		t.Errorf("morceaux = %d, want %d", len(tracks), PageSize+PageSize+PageSize/2)
	}
}

func TestFetchAllTracksRepeatedPage(t *testing.T) {
	// L'API embed IGNORE l'offset : toutes les pages renvoient la même.
	fetch := func(string) ([]byte, error) { return pageFixture(PageSize), nil }
	tracks, truncated, err := FetchAllTracks("https://open.spotify.com/playlist/abc123", fetch)
	if err != nil {
		t.Fatal(err)
	}
	if !truncated {
		t.Error("truncated = false, want true (page répétée → liste plafonnée)")
	}
	if len(tracks) != PageSize {
		t.Errorf("morceaux = %d, want %d (la page répétée n'est comptée qu'une fois)", len(tracks), PageSize)
	}
}

func TestFetchAllTracksMaxPages(t *testing.T) {
	// 20 pages pleines sans page courte : on s'arrête au cap MAX_PAGES.
	tracks, truncated, err := FetchAllTracks("https://open.spotify.com/playlist/abc123", fakePages(PageSize, MaxPages))
	if err != nil {
		t.Fatal(err)
	}
	if !truncated {
		t.Error("truncated = false, want true (cap MAX_PAGES atteint)")
	}
	if len(tracks) != PageSize*MaxPages {
		t.Errorf("morceaux = %d, want %d (safety cap 2000)", len(tracks), PageSize*MaxPages)
	}
}

func TestFetchAllTracksTrack(t *testing.T) {
	html, _ := os.ReadFile(filepath.Join("testdata", "track.html"))
	fetch := func(string) ([]byte, error) { return html, nil }
	tracks, truncated, err := FetchAllTracks("https://open.spotify.com/track/xyz", fetch)
	if err != nil {
		t.Fatal(err)
	}
	if truncated || len(tracks) != 1 || tracks[0].Title != "La puerta" {
		t.Errorf("track = %+v, truncated=%v", tracks, truncated)
	}
}

func TestFetchAllTracksInvalidURL(t *testing.T) {
	tracks, truncated, err := FetchAllTracks("https://example.com/not-a-media-link", nil)
	if err != nil || tracks != nil || truncated {
		t.Errorf("lien invalide = %v, %v, truncated=%v; want nil, nil, false", tracks, err, truncated)
	}
}

// ── Filter (lookup O(1) dans le set) ─────────────────────────────────────

func TestFilter(t *testing.T) {
	// Les clés sont des NOMS DE FICHIERS normalisés (ce que produit
	// ExistingSet) : on les construit avec Normalize pour être exacts.
	existing := map[string]struct{}{
		Normalize("Ninho - Vrais"):    {},
		Normalize("Jul - Zone"):       {},
		Normalize("Naps - Par Amour"): {},
	}
	tracks := []Track{
		{Title: "Vrais", Artist: "Ninho", Query: "Ninho - Vrais"},
		{Title: "Inédit", Artist: "Ninho", Query: "Ninho - Inédit"},
		{Title: "Zone", Artist: "Jul", Query: "Jul - Zone"},
	}
	skipped, missing := Filter(existing, tracks)
	if len(skipped) != 2 {
		t.Errorf("skipped = %v, want 2", skipped)
	}
	if len(missing) != 1 || missing[0] != "Ninho - Inédit" {
		t.Errorf("missing = %v, want [Ninho - Inédit]", missing)
	}
}

// ── Run (pipeline complet) ───────────────────────────────────────────────

func TestRunApplied(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "Vrais.mp3"), []byte("x"), 0o644)
	fetch := func(string) ([]byte, error) { return pageFixture(PageSize / 2), nil }
	res := Run(dir, "https://open.spotify.com/playlist/abc", fetch)

	if !res.Applied {
		t.Fatalf("Applied = false, raison: %q", res.Reason)
	}
	if res.TotalTracks != PageSize/2 {
		t.Errorf("TotalTracks = %d, want %d", res.TotalTracks, PageSize/2)
	}
	// « Song 000 » n'est pas sur disque (le fichier Vrais.mp3 ne correspond
	// à aucun titre de la page) → tout est manquant.
	if res.MissingCount != PageSize/2 || res.AlreadyDownloaded != 0 {
		t.Errorf("missing = %d, already = %d; want %d et 0", res.MissingCount, res.AlreadyDownloaded, PageSize/2)
	}
	if len(res.MissingQueries) != PageSize/2 || len(res.SkippedTracks) != 0 {
		t.Errorf("missing_queries = %d, skipped = %d", len(res.MissingQueries), len(res.SkippedTracks))
	}
}

func TestRunAllPresent(t *testing.T) {
	dir := t.TempDir()
	// Fichiers « Artist - Title.mp3 » : le nom normalisé = requête normalisée.
	_ = os.WriteFile(filepath.Join(dir, "Artist 000 - Song 000.mp3"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "Artist 001 - Song 001.mp3.mp3"), []byte("x"), 0o644)
	fetch := func(string) ([]byte, error) { return pageFixture(2), nil }
	res := Run(dir, "https://open.spotify.com/album/def", fetch)

	if !res.Applied || res.MissingCount != 0 || res.AlreadyDownloaded != 2 {
		t.Errorf("résultat = %+v, want applied avec 2 déjà sur disque et 0 manquant", res)
	}
}

func TestRunTruncated(t *testing.T) {
	dir := t.TempDir()
	fetch := func(string) ([]byte, error) { return pageFixture(PageSize), nil } // page répétée
	res := Run(dir, "https://open.spotify.com/playlist/abc", fetch)
	if res.Applied {
		t.Error("Applied = true, want false (liste plafonnée)")
	}
	if !strings.Contains(res.Reason, "limité") {
		t.Errorf("reason = %q, want mention du plafond embed", res.Reason)
	}
}

func TestRunNoTracks(t *testing.T) {
	dir := t.TempDir()
	fetch := func(string) ([]byte, error) { return nil, nil }
	res := Run(dir, "https://open.spotify.com/playlist/abc", fetch)
	if res.Applied || res.Reason != "No tracks extracted via embed API" {
		t.Errorf("résultat = %+v, want filtre désactivé sans raison", res)
	}
}
