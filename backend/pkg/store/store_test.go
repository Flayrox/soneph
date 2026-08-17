package store

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"soneph-backend/pkg/storage"
)

// openTest ouvre une base SQLite sur un fichier temporaire réel (WAL actif,
// migrations appliquées) — jamais en mémoire, pour couvrir le vrai chemin
// de persistance.
func openTest(t *testing.T) *SQLiteStore {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "soneph.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// file construit une storage.DownloadedFile pour les scénarios de test.
func file(relPath, title, artist, album string, modTime time.Time) storage.DownloadedFile {
	return storage.DownloadedFile{
		FileName:   filepath.Base(relPath),
		Title:      title,
		Artist:     artist,
		Album:      album,
		RelPath:    relPath,
		Size:       int64(len(title)),
		LyricsType: "none",
		ModTime:    modTime,
	}
}

func TestOpenAppliesMigrations(t *testing.T) {
	st := openTest(t)

	cases := []struct {
		table string
	}{
		{"artists"}, {"albums"}, {"tracks"}, {"playlists"},
		{"playlist_tracks"}, {"likes"}, {"history"}, {"jobs"},
		{"pins"}, {"settings"}, {"tracks_fts"},
	}
	for _, c := range cases {
		var n int
		err := st.db.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type IN ('table','virtual table') AND name = ?`, c.table,
		).Scan(&n)
		if err != nil {
			t.Fatalf("vérification table %s: %v", c.table, err)
		}
		if n != 1 {
			t.Errorf("table %s absente après migration", c.table)
		}
	}

	// Version goose présente (migrations appliquées).
	var version int64
	if err := st.db.QueryRow(`SELECT MAX(version_id) FROM goose_db_version`).Scan(&version); err != nil {
		t.Fatalf("goose_db_version: %v", err)
	}
	if version != 2 {
		t.Errorf("version goose = %d, want 2", version)
	}
}

func TestSyncLibraryAndList(t *testing.T) {
	st := openTest(t)
	base := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)

	stats, err := st.SyncLibrary([]storage.DownloadedFile{
		file("artists/radiohead/ok-computer/01-airbag.mp3", "Airbag", "Radiohead", "OK Computer", base),
		file("artists/radiohead/ok-computer/02-paranoid-android.mp3", "Paranoid Android", "Radiohead", "OK Computer", base),
		file("artists/daft-punk/homework/01-daftendirekt.mp3", "Daftendirekt", "Daft Punk", "Homework", base.Add(time.Hour)),
	})
	if err != nil {
		t.Fatalf("SyncLibrary: %v", err)
	}
	if stats.Scanned != 3 || stats.Added != 3 || stats.Updated != 0 || stats.Unchanged != 0 {
		t.Errorf("stats = %+v, want 3 ajoutés", stats)
	}

	n, err := st.CountTracks()
	if err != nil || n != 3 {
		t.Fatalf("CountTracks = %d, %v; want 3", n, err)
	}

	tracks, err := st.ListTracks(100, 0)
	if err != nil {
		t.Fatalf("ListTracks: %v", err)
	}
	if len(tracks) != 3 {
		t.Fatalf("ListTracks = %d morceaux, want 3", len(tracks))
	}
	// Tri par artiste, album, piste.
	if tracks[0].Artist != "Daft Punk" || tracks[0].Title != "Daftendirekt" {
		t.Errorf("tracks[0] = %+v (tri attendu : Daft Punk d'abord)", tracks[0])
	}
	if tracks[2].Artist != "Radiohead" || tracks[2].Album != "OK Computer" {
		t.Errorf("tracks[2] = %+v", tracks[2])
	}

	// TrackByPath résout artiste + album.
	tr, err := st.TrackByPath("artists/radiohead/ok-computer/01-airbag.mp3")
	if err != nil {
		t.Fatalf("TrackByPath: %v", err)
	}
	if tr.Artist != "Radiohead" || tr.Album != "OK Computer" || !tr.UpdatedAt.Equal(base) {
		t.Errorf("TrackByPath = %+v", tr)
	}
}

func TestSyncLibraryDelta(t *testing.T) {
	st := openTest(t)
	base := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
	files := []storage.DownloadedFile{
		file("a.mp3", "Titre", "Artiste", "Album", base),
		file("b.mp3", "Autre", "Artiste", "Album", base.Add(time.Minute)),
	}

	first, err := st.SyncLibrary(files)
	if err != nil {
		t.Fatalf("premier sync: %v", err)
	}
	second, err := st.SyncLibrary(files)
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}

	if second.Added != 0 || second.Updated != 0 || second.Unchanged != 2 {
		t.Errorf("second sync = %+v, want 2 inchangés, 0 écritures", second)
	}
	if first.Added != 2 {
		t.Errorf("premier sync = %+v, want 2 ajoutés", first)
	}
}

func TestSyncLibraryUpdate(t *testing.T) {
	st := openTest(t)
	base := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
	f := file("a.mp3", "Ancien titre", "Artiste", "Album", base)

	if _, err := st.SyncLibrary([]storage.DownloadedFile{f}); err != nil {
		t.Fatalf("premier sync: %v", err)
	}

	// Le fichier change (nouveau mtime + nouveau titre).
	f.Title = "Nouveau titre"
	f.ModTime = base.Add(2 * time.Hour)
	stats, err := st.SyncLibrary([]storage.DownloadedFile{f})
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if stats.Updated != 1 || stats.Added != 0 {
		t.Errorf("stats = %+v, want 1 mis à jour", stats)
	}

	tr, err := st.TrackByPath("a.mp3")
	if err != nil {
		t.Fatalf("TrackByPath: %v", err)
	}
	if tr.Title != "Nouveau titre" {
		t.Errorf("titre = %q, want %q", tr.Title, "Nouveau titre")
	}

	// Un seul morceau, pas de doublon.
	if n, _ := st.CountTracks(); n != 1 {
		t.Errorf("CountTracks = %d, want 1 (pas de doublon)", n)
	}

	// L'index FTS ne doit pas garder l'ancienne entrée (aucun doublon au
	// résultat de recherche après une mise à jour).
	got, err := st.SearchTracks("Nouveau titre", 10)
	if err != nil {
		t.Fatalf("SearchTracks: %v", err)
	}
	if len(got) != 1 || got[0].Title != "Nouveau titre" {
		t.Errorf("recherche après maj = %v, want exactement [Nouveau titre]", got)
	}
}

func TestSearchTracks(t *testing.T) {
	st := openTest(t)
	base := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
	if _, err := st.SyncLibrary([]storage.DownloadedFile{
		file("01-airbag.mp3", "Airbag", "Radiohead", "OK Computer", base),
		file("02-karma.mp3", "Karma Police", "Radiohead", "OK Computer", base),
		file("03-touch.mp3", "Touch", "Daft Punk", "Random Access Memories", base.Add(time.Hour)),
		file("04-harder.mp3", "Harder, Better, Faster, Stronger", "Daft Punk", "Homework", base.Add(2*time.Hour)),
	}); err != nil {
		t.Fatalf("SyncLibrary: %v", err)
	}

	cases := []struct {
		name  string
		query string
		want  []string // titres attendus, dans l'ordre
	}{
		{"titre exact", "Airbag", []string{"Airbag"}},
		{"préfixe de titre", "Kar", []string{"Karma Police"}},
		{"artiste", "Radiohead", []string{"Airbag", "Karma Police"}},
		{"album", "Homework", []string{"Harder, Better, Faster, Stronger"}},
		{"multi-mots", "Daft harder", []string{"Harder, Better, Faster, Stronger"}},
		{"aucun résultat", "zzz", []string{}},
		{"requête vide", "", []string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := st.SearchTracks(c.query, 50)
			if err != nil {
				t.Fatalf("SearchTracks(%q): %v", c.query, err)
			}
			var titles []string
			for _, tr := range got {
				titles = append(titles, tr.Title)
			}
			if len(titles) != len(c.want) {
				t.Fatalf("SearchTracks(%q) = %v, want %v", c.query, titles, c.want)
			}
			for i := range titles {
				if titles[i] != c.want[i] {
					t.Errorf("SearchTracks(%q)[%d] = %q, want %q", c.query, i, titles[i], c.want[i])
				}
			}
		})
	}
}

// TestSearchTracksPerformance couvre la DoD M2 : « FTS search <50ms sur 1k+
// morceaux ». Le seuil d'assertion est large (1s) pour rester stable en CI ;
// la valeur mesurée est loggée pour le rapport.
func TestSearchTracksPerformance(t *testing.T) {
	st := openTest(t)
	base := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)

	const n = 1500
	files := make([]storage.DownloadedFile, 0, n)
	for i := 0; i < n; i++ {
		files = append(files, file(
			fmt.Sprintf("artists/artist-%04d/album/01-track-%04d.mp3", i%100, i),
			fmt.Sprintf("Track %04d", i),
			fmt.Sprintf("Artist %04d", i%100),
			fmt.Sprintf("Album %04d", i/100),
			base.Add(time.Duration(i)*time.Second),
		))
	}
	if _, err := st.SyncLibrary(files); err != nil {
		t.Fatalf("SyncLibrary: %v", err)
	}
	if cnt, _ := st.CountTracks(); cnt != n {
		t.Fatalf("CountTracks = %d, want %d", cnt, n)
	}

	start := time.Now()
	got, err := st.SearchTracks("Track 1234", 20)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("SearchTracks: %v", err)
	}
	t.Logf("recherche FTS sur %d morceaux : %v (DoD : <50ms)", n, elapsed)
	if len(got) == 0 || got[0].Title != "Track 1234" {
		t.Errorf("premier résultat = %+v, want Track 1234", got)
	}
	if elapsed > time.Second {
		t.Errorf("recherche trop lente : %v", elapsed)
	}
}

func TestSettings(t *testing.T) {
	st := openTest(t)

	if _, err := st.GetSetting("workers"); err != ErrNotFound {
		t.Fatalf("GetSetting inexistant = %v, want ErrNotFound", err)
	}
	if err := st.SetSetting("workers", "8"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	v, err := st.GetSetting("workers")
	if err != nil || v != "8" {
		t.Fatalf("GetSetting = %q, %v; want 8", v, err)
	}
	// Mise à jour (upsert, pas de doublon).
	if err := st.SetSetting("workers", "12"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	v, _ = st.GetSetting("workers")
	if v != "12" {
		t.Errorf("GetSetting après maj = %q, want 12", v)
	}
}

func TestJobs(t *testing.T) {
	st := openTest(t)

	create := []Job{
		{ID: "j1", Type: "download", Payload: `{"url":"https://example.com/a"}`, Status: "queued", Priority: 0},
		{ID: "j2", Type: "lyrics", Payload: `{}`, Status: "queued", Priority: 5},
		{ID: "j3", Type: "rescan", Payload: `{}`, Status: "done"},
	}
	for _, j := range create {
		if err := st.CreateJob(j); err != nil {
			t.Fatalf("CreateJob(%s): %v", j.ID, err)
		}
	}

	queued, err := st.ListJobs("queued", 10)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(queued) != 2 {
		t.Fatalf("ListJobs(queued) = %d, want 2", len(queued))
	}
	// Tri : priorité décroissante.
	if queued[0].ID != "j2" {
		t.Errorf("premier job = %s, want j2 (priorité 5)", queued[0].ID)
	}

	if err := st.UpdateJobStatus("j1", "running", ""); err != nil {
		t.Fatalf("UpdateJobStatus(running): %v", err)
	}
	if err := st.UpdateJobStatus("j1", "done", ""); err != nil {
		t.Fatalf("UpdateJobStatus(done): %v", err)
	}

	j1, err := st.ListJobs("done", 10)
	if err != nil {
		t.Fatalf("ListJobs(done): %v", err)
	}
	found := false
	for _, j := range j1 {
		if j.ID == "j1" {
			found = true
			if j.FinishedAt == nil {
				t.Errorf("job j1 terminé sans finished_at")
			}
		}
	}
	if !found {
		t.Errorf("job j1 absent de la liste des jobs done")
	}

	// Statut inconnu → ErrNotFound.
	if err := st.UpdateJobStatus("nope", "done", ""); err != ErrNotFound {
		t.Errorf("UpdateJobStatus inexistant = %v, want ErrNotFound", err)
	}
}

func TestFtsQuery(t *testing.T) {
	cases := []struct{ in, want string }{
		{"airbag", `"airbag"*`},
		{"Radiohead OK", `"radiohead"* AND "ok"*`},
		{"  espaces   multiples  ", `"espaces"* AND "multiples"*`},
		{"sym$boles!et'ponctuation", `"sym"* AND "boles"* AND "et"* AND "ponctuation"*`},
		{"", ""},
		{"!!!", ""},
	}
	for _, c := range cases {
		if got := ftsQuery(c.in); got != c.want {
			t.Errorf("ftsQuery(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	// Pas d'injection d'opérateur FTS5 : tout caractère non alphanumérique
	// est supprimé, les mots sont ré-émis en préfixes quotés joints par AND.
	nasty := []struct{ in, want string }{
		{`"`, ``},
		{`a"b`, `"a"* AND "b"*`},
		{`a OR b`, `"a"* AND "or"* AND "b"*`},
		{`NOT a`, `"not"* AND "a"*`},
		{`*`, ``},
		{"a--b;DROP TABLE tracks", `"a"* AND "b"* AND "drop"* AND "table"* AND "tracks"*`},
	}
	for _, c := range nasty {
		if got := ftsQuery(c.in); got != c.want {
			t.Errorf("ftsQuery(%q) = %q, want %q (injection possible ?)", c.in, got, c.want)
		}
	}
}
