package handler

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"soneph-backend/pkg/downloader"
	"soneph-backend/pkg/storage"
	"soneph-backend/pkg/store"
	"soneph-backend/pkg/syncmgr"

	"github.com/gin-gonic/gin"
)

func TestIsPlaylistLink(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M", true},
		{"spotify:playlist:37i9dQZF1DXcBWIGoYBM5M", true},
		{"https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M?si=abc123", true},
		{"https://open.spotify.com/album/0hl8Tcf7Ik3", false},
		{"https://open.spotify.com/track/5ojN1zP5JN", false},
		{"https://open.spotify.com/artist/6fxyWrfmjcbj5d12gXeiNV", false},
		{"https://open.spotify.com/playlist/", true},
		{"", false},
	}
	for _, c := range cases {
		if got := isPlaylistLink(c.url); got != c.want {
			t.Errorf("isPlaylistLink(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

// newTestAPI construit une API complète sur des répertoires temporaires
// (scanner, moteur, stores JSON isolés, auth désactivée) et un routeur gin
// avec toutes les routes enregistrées — prête pour des tests de bout en bout
// sans toucher à la config utilisateur ni au réseau.
func newTestAPI(t *testing.T) (*gin.Engine, *API) {
	t.Helper()
	dir := t.TempDir()
	// Isoler tous les emplacements d'état (fichiers JSON, config, file du
	// moteur) pour ne jamais lire/écrire la config réelle de la machine.
	t.Setenv("SONEPH_TOKEN", "")
	t.Setenv("SONEPH_HISTORY_FILE", filepath.Join(dir, "history.json"))
	t.Setenv("SONEPH_LIKES_FILE", filepath.Join(dir, "likes.json"))
	t.Setenv("SONEPH_PLAYLISTS_DIR", filepath.Join(dir, "playlists"))
	t.Setenv("SONEPH_CONFIG", filepath.Join(dir, "settings.json"))
	t.Setenv("SONEPH_QUEUE_FILE", filepath.Join(dir, "queue.json"))

	downloadDir := filepath.Join(dir, "downloads")
	hub := NewWSHub()
	dl := downloader.NewManager(downloadDir, hub.Broadcast)
	sc := storage.NewScanner(downloadDir)
	imp := syncmgr.New(downloadDir)
	st, err := store.Open(filepath.Join(dir, "soneph.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	api := NewAPI(dl, sc, imp, st, hub)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	api.RegisterRoutes(r)
	return r, api
}

// TestRegisterRoutes couvre chaque endpoint de RegisterRoutes : code de
// statut attendu (succès ET erreurs de validation) et présence des clés JSON
// contractuelles. Verrouille la DoD M1 (« endpoints identiques ») dans CI.
func TestRegisterRoutes(t *testing.T) {
	r, _ := newTestAPI(t)
	exportDir := filepath.Join(t.TempDir(), "export")

	cases := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantKeys   []string // clés JSON contractuelles pour les succès
	}{
		// ── downloads ────────────────────────────────────────────────
		{"download - body invalide", http.MethodPost, "/api/download", `{}`, http.StatusBadRequest, nil},
		{"download - URL acceptée", http.MethodPost, "/api/download", `{"url":"https://example.com/track"}`, http.StatusAccepted, []string{"message", "task"}},
		{"tasks", http.MethodGet, "/api/tasks", "", http.StatusOK, []string{"tasks"}},
		{"jobs - file vide", http.MethodGet, "/api/jobs", "", http.StatusOK, []string{"jobs"}},
		{"downloads - bibliothèque vide", http.MethodGet, "/api/downloads", "", http.StatusOK, []string{"files"}},
		{"library - base vide", http.MethodGet, "/api/library", "", http.StatusOK, []string{"count", "tracks"}},
		{"rescan - base vide", http.MethodPost, "/api/rescan", "", http.StatusOK, []string{"stats"}},
		{"search sans q", http.MethodGet, "/api/search", "", http.StatusOK, []string{"query", "tracks"}},
		{"search avec q", http.MethodGet, "/api/search?q=airbag", "", http.StatusOK, []string{"query", "tracks"}},
		{"delete download sans path", http.MethodDelete, "/api/downloads", "", http.StatusBadRequest, nil},
		// Quirk existant : DeleteDownload ne mappe pas ErrInvalidPath sur 400
		// (contrairement aux autres handlers) — le test fige le comportement actuel.
		{"delete download chemin invalide", http.MethodDelete, "/api/downloads?path=../../etc/passwd", "", http.StatusInternalServerError, nil},

		// ── tracks ───────────────────────────────────────────────────
		{"stream sans path", http.MethodGet, "/api/stream", "", http.StatusBadRequest, nil},
		{"stream chemin invalide", http.MethodGet, "/api/stream?path=../../etc/passwd", "", http.StatusBadRequest, nil},
		{"file/details sans path", http.MethodGet, "/api/file/details", "", http.StatusBadRequest, nil},
		{"file/details chemin invalide", http.MethodGet, "/api/file/details?path=../../etc/passwd", "", http.StatusBadRequest, nil},
		{"cover sans path", http.MethodGet, "/api/cover", "", http.StatusBadRequest, nil},
		{"cover chemin invalide", http.MethodGet, "/api/cover?path=../../etc/passwd", "", http.StatusBadRequest, nil},
		{"lyrics sans path", http.MethodGet, "/api/lyrics", "", http.StatusBadRequest, nil},

		// ── lyrics retry ─────────────────────────────────────────────
		{"lyrics/retry - job démarré", http.MethodPost, "/api/lyrics/retry", "", http.StatusAccepted, []string{"message", "job"}},
		{"lyrics/retry - statut", http.MethodGet, "/api/lyrics/retry", "", http.StatusOK, []string{"job"}},

		// ── system ───────────────────────────────────────────────────
		{"settings", http.MethodGet, "/api/settings", "", http.StatusOK, []string{"workers", "threads"}},
		{"settings enregistrés", http.MethodPost, "/api/settings", `{"workers":2,"threads":3}`, http.StatusOK, []string{"message", "settings"}},
		{"settings invalides", http.MethodPost, "/api/settings", `{"workers":"x"}`, http.StatusBadRequest, nil},

		// ── playlists ────────────────────────────────────────────────
		{"playlists - liste vide", http.MethodGet, "/api/playlists", "", http.StatusOK, []string{"playlists"}},
		{"playlist créée", http.MethodPost, "/api/playlists", `{"name":"Roadtrip"}`, http.StatusCreated, []string{"playlist"}},
		// Quirk existant : un name vide crée quand même une playlist (le store
		// la nomme « Playlist ») — le test fige le comportement actuel.
		{"playlist sans name - nommée par défaut", http.MethodPost, "/api/playlists", `{}`, http.StatusCreated, []string{"playlist"}},
		{"playlist introuvable", http.MethodGet, "/api/playlists/pl_zzz", "", http.StatusNotFound, nil},
		{"delete playlist introuvable", http.MethodDelete, "/api/playlists/pl_zzz", "", http.StatusNotFound, nil},
		{"add track playlist introuvable", http.MethodPost, "/api/playlists/pl_zzz/tracks", `{"path":"a.mp3"}`, http.StatusNotFound, nil},
		{"remove track playlist introuvable", http.MethodDelete, "/api/playlists/pl_zzz/tracks?path=a.mp3", "", http.StatusNotFound, nil},
		{"reorder playlist introuvable", http.MethodPost, "/api/playlists/pl_zzz/order", `{"paths":["a.mp3"]}`, http.StatusNotFound, nil},
		{"playlists/export - aucune playlist", http.MethodPost, "/api/playlists/export", `{"dir":"` + exportDir + `"}`, http.StatusOK, []string{"dir", "files", "count"}},
		{"playlists/export sans dir", http.MethodPost, "/api/playlists/export", `{}`, http.StatusBadRequest, nil},

		// ── history / stats / likes ──────────────────────────────────
		{"scrobble enregistré", http.MethodPost, "/api/scrobble", `{"path":"a.mp3","duration":180}`, http.StatusOK, []string{"message"}},
		{"scrobble sans path", http.MethodPost, "/api/scrobble", `{}`, http.StatusBadRequest, nil},
		{"history/recent", http.MethodGet, "/api/history/recent", "", http.StatusOK, []string{"history"}},
		{"history/top", http.MethodGet, "/api/history/top", "", http.StatusOK, []string{"top"}},
		{"stats", http.MethodGet, "/api/stats", "", http.StatusOK, []string{}},
		{"likes - liste", http.MethodGet, "/api/likes", "", http.StatusOK, []string{"likes"}},
		{"like ajouté", http.MethodPost, "/api/likes", `{"path":"a.mp3"}`, http.StatusOK, []string{"message"}},
		{"like sans path", http.MethodPost, "/api/likes", `{}`, http.StatusBadRequest, nil},
		{"unlike sans path", http.MethodDelete, "/api/likes", "", http.StatusBadRequest, nil},
		{"unlike", http.MethodDelete, "/api/likes?path=a.mp3", "", http.StatusOK, []string{"message"}},

		// ── pins / queue (M3) ────────────────────────────────────────
		{"pins - liste vide", http.MethodGet, "/api/pins", "", http.StatusOK, []string{"pins"}},
		{"pin ajouté", http.MethodPost, "/api/pins", `{"kind":"artist","value":"Radiohead"}`, http.StatusCreated, []string{"message"}},
		{"pin kind invalide", http.MethodPost, "/api/pins", `{"kind":"song","value":"x"}`, http.StatusBadRequest, nil},
		{"pin sans value", http.MethodPost, "/api/pins", `{"kind":"artist"}`, http.StatusBadRequest, nil},
		{"unpin sans params", http.MethodDelete, "/api/pins", "", http.StatusBadRequest, nil},
		{"unpin", http.MethodDelete, "/api/pins?kind=artist&value=Radiohead", "", http.StatusOK, []string{"message"}},
		{"queue - vide", http.MethodGet, "/api/queue", "", http.StatusOK, []string{"queue", "index"}},
		{"queue enregistrée", http.MethodPut, "/api/queue", `{"queue":["a.mp3","b.mp3"],"index":1}`, http.StatusOK, []string{"message"}},

		// ── sync ─────────────────────────────────────────────────────
		{"sync/status", http.MethodGet, "/api/sync/status", "", http.StatusOK, []string{"platform", "downloads_dir"}},
		// Le watcher n'existe que sur macOS avec scripts/watch_and_import.sh :
		// en environnement de test il est toujours indisponible → 409.
		{"sync/start - watcher indisponible", http.MethodPost, "/api/sync/start", "", http.StatusConflict, nil},
		{"sync/stop", http.MethodPost, "/api/sync/stop", "", http.StatusOK, nil},

		// ── library / dedup ──────────────────────────────────────────
		{"duplicates", http.MethodGet, "/api/duplicates", "", http.StatusOK, []string{"groups", "total"}},
		{"duplicates/remove sans paths", http.MethodPost, "/api/duplicates/remove", `{}`, http.StatusBadRequest, nil},

		// ── websocket ────────────────────────────────────────────────
		// Un GET HTTP classique sur /ws n'est pas un handshake WebSocket.
		{"ws - handshake HTTP refusé", http.MethodGet, "/api/ws", "", http.StatusBadRequest, nil},

		// ── jobs (M4 / M5) ──────────────────────────────────────────────
		{"jobs - file non vide", http.MethodGet, "/api/jobs", "", http.StatusOK, []string{"jobs"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var body io.Reader
			if tc.body != "" {
				body = strings.NewReader(tc.body)
			}
			req := httptest.NewRequest(tc.method, tc.path, body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("%s %s → %d, want %d (body: %s)", tc.method, tc.path, w.Code, tc.wantStatus, w.Body.String())
			}
			if len(tc.wantKeys) == 0 {
				return
			}
			var resp map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("%s %s → réponse non-JSON : %v (body: %s)", tc.method, tc.path, err, w.Body.String())
			}
			for _, k := range tc.wantKeys {
				if _, ok := resp[k]; !ok {
					t.Errorf("%s %s → clé JSON %q absente (body: %s)", tc.method, tc.path, k, w.Body.String())
				}
			}
		})
	}
}

// ── M6 : /api/file/details et /api/cover sans Python ────────────────────
// Le panneau « Plus de détails » lit les tags en Go (pkg/tags) : on vérifie
// le parcours complet avec un vrai MP3 taggé (fichier + réponse JSON) et
// l'extraction de pochette (APIC) avec cache.

// buildTaggedMP3Handler écrit un MP3 ID3v2.3 (TIT2/TPE1/TALB/WOAS/TXXX +
// APIC + frames MPEG 128 kbps) sous downloadDir/<relPath>.
func buildTaggedMP3Handler(t *testing.T, downloadDir, relPath string) string {
	t.Helper()
	text := func(id, s string) []byte {
		d := append([]byte{0}, []byte(s)...)
		out := append([]byte(id), byte(len(d)>>24), byte(len(d)>>16), byte(len(d)>>8), byte(len(d)))
		return append(append(out, 0, 0), d...)
	}
	urlF := func(id, s string) []byte {
		out := append([]byte(id), byte(len(s)>>24), byte(len(s)>>16), byte(len(s)>>8), byte(len(s)))
		return append(append(out, 0, 0), s...)
	}
	txxx := func(desc, val string) []byte {
		d := append([]byte{0}, []byte(desc)...)
		d = append(d, 0)
		d = append(d, []byte(val)...)
		out := append([]byte("TXXX"), byte(len(d)>>24), byte(len(d)>>16), byte(len(d)>>8), byte(len(d)))
		return append(append(out, 0, 0), d...)
	}
	cover := []byte("\xff\xd8\xff\xe0JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00\xff\xd9")
	apic := append([]byte{0}, []byte("image/jpeg")...)
	apic = append(apic, 0, 3, 0)
	apic = append(apic, cover...)
	apicF := append([]byte("APIC"), byte(len(apic)>>24), byte(len(apic)>>16), byte(len(apic)>>8), byte(len(apic)))
	apicF = append(append(apicF, 0, 0), apic...)

	var body []byte
	for _, f := range [][]byte{
		text("TIT2", "Vrais"),
		text("TPE1", "Ninho"),
		text("TALB", "M.I.L.S 2.0"),
		urlF("WOAS", "https://open.spotify.com/track/aaa"),
		txxx("SONEPH_SOURCE", "https://open.spotify.com/track/aaa"),
		apicF,
	} {
		body = append(body, f...)
	}
	header := []byte{'I', 'D', '3', 3, 0, 0,
		byte(len(body) >> 21 & 0x7F), byte(len(body) >> 14 & 0x7F), byte(len(body) >> 7 & 0x7F), byte(len(body) & 0x7F)}

	frameLen := 144*128000/44100 - 4
	var audio []byte
	for i := 0; i < 40; i++ {
		audio = append(audio, 0xFF, 0xFB, 0x90, 0x00)
		audio = append(audio, make([]byte, frameLen)...)
	}

	full := filepath.Join(downloadDir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, append(append(header, body...), audio...), 0o644); err != nil {
		t.Fatalf("écriture du MP3: %v", err)
	}
	return full
}

func TestFileDetailsAndCoverGo(t *testing.T) {
	r, api := newTestAPI(t)
	rel := "Ninho/Vrais.mp3"
	buildTaggedMP3Handler(t, api.scanner.DownloadDir, rel)

	// ── /api/file/details : JSON complet sans sous-processus Python ──
	req := httptest.NewRequest(http.MethodGet, "/api/file/details?path="+rel, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("file/details → %d (body: %s)", w.Code, w.Body.String())
	}
	var d map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]interface{}{
		"title":       "Vrais",
		"artist":      "Ninho",
		"album":       "M.I.L.S 2.0",
		"spotify_url": "https://open.spotify.com/track/aaa",
		"source_url":  "https://open.spotify.com/track/aaa",
		"bitrate":     "128kbps",
		"quality":     "128kbps", // pas de SONEPH_QUALITY → bitrate réel
	} {
		if d[key] != want {
			t.Errorf("details[%q] = %#v, want %#v", key, d[key], want)
		}
	}

	// ── /api/cover : pochette extraite + cache .covers ──
	req = httptest.NewRequest(http.MethodGet, "/api/cover?path="+rel, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("cover → %d (body: %s)", w.Code, w.Body.String())
	}
	h := md5.Sum([]byte(rel))
	cachePath := filepath.Join(api.scanner.DownloadDir, ".covers", hex.EncodeToString(h[:])+".jpg")
	if _, err := os.Stat(cachePath); err != nil {
		t.Errorf("cache pochette absent : %v", err)
	}
	if len(w.Body.Bytes()) == 0 || w.Body.Bytes()[0] != 0xFF || w.Body.Bytes()[1] != 0xD8 {
		t.Errorf("corps de la cover inattendu (%d octets)", len(w.Body.Bytes()))
	}
}

// TestGetJobsWithRetry vérifie que GET /api/jobs expose ce dont le panneau
// frontend a besoin : statut, type, tentatives et retry_at (compte à
// rebours du backoff M4).
func TestGetJobsWithRetry(t *testing.T) {
	r, api := newTestAPI(t)

	// Un job download clôturé et un job fast_filter en backoff (retry_at).
	if err := api.st.CreateJob(store.Job{ID: "j_done", Type: "download", Payload: `{"url":"https://example.com/a"}`, Status: "done"}); err != nil {
		t.Fatalf("CreateJob(done): %v", err)
	}
	if err := api.st.CreateJob(store.Job{ID: "ff_1", Type: "fast_filter", Payload: `{"task_id":"t1","url":"https://example.com/b"}`, Status: "queued"}); err != nil {
		t.Fatalf("CreateJob(ff): %v", err)
	}
	if err := api.st.SetRetryAt("ff_1", time.Now().Add(30*time.Second)); err != nil {
		t.Fatalf("SetRetryAt: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/jobs → %d (body: %s)", w.Code, w.Body.String())
	}
	var resp struct {
		Jobs []store.Job `json:"jobs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Jobs) != 2 {
		t.Fatalf("jobs = %d, want 2", len(resp.Jobs))
	}
	var ff *store.Job
	for i := range resp.Jobs {
		if resp.Jobs[i].ID == "ff_1" {
			ff = &resp.Jobs[i]
		}
	}
	if ff == nil {
		t.Fatal("job fast_filter absent de la réponse")
	}
	if ff.Type != "fast_filter" || ff.Status != "queued" {
		t.Errorf("ff = type %q status %q", ff.Type, ff.Status)
	}
	if ff.RetryAt == nil {
		t.Error("retry_at absent — le panneau frontend ne peut pas afficher le compte à rebours")
	} else if left := time.Until(*ff.RetryAt); left > 40*time.Second || left < 20*time.Second {
		t.Errorf("retry_at = %v (dans %v), want ~30 s", *ff.RetryAt, left)
	}
}
