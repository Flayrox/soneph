// Package fastfilter est le port Go de fast_filter.py : il compare la liste
// des morceaux d'un lien (playlist/album/track via l'API embed) à ce qui est
// déjà sur disque, pour afficher instantanément « X déjà sur disque, Y à
// télécharger » avant même de lancer le moteur de téléchargement.
//
// Les choix du script d'origine sont conservés à l'identique :
//   - lookup O(1) dans un set de noms normalisés (au lieu d'un parcours
//     O(N×M) de toute la bibliothèque par titre de playlist) ;
//   - pagination parallèle de l'API embed (100 titres/page, MAX_PAGES=20) ;
//   - détection des pages répétées : l'API embed IGNORE le paramètre offset
//     au-delà de ~100 titres et renvoie les mêmes morceaux — on le détecte
//     et on désactive le filtre (truncated) plutôt que de compter faux.
//
// La fonction de fetch est injectable (FetchFunc) : les tests utilisent des
// fixtures locales, aucun appel réseau.
package fastfilter

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// PageSize : nombre de titres renvoyés par l'API embed par page.
	PageSize = 100
	// MaxPages borne la pagination (safety cap : 20 × 100 = 2000 titres).
	MaxPages = 20
	// maxWorkers : nombre de pages téléchargées en parallèle.
	maxWorkers = 8
	// userAgent est le même en-tête que le script Python.
	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)"
)

var (
	reBracket = regexp.MustCompile(`[\(\[\{].*?[\)\]\}]`)
	reNonWord = regexp.MustCompile(`[^\p{L}\p{N}_\s]`)
	// (?s) : le JSON peut contenir des retours à la ligne (comme re.DOTALL
	// du script Python).
	reNextData = regexp.MustCompile(`<script id="__NEXT_DATA__" type="application/json">(?s)(.*?)</script>`)
	reMedia    = regexp.MustCompile(`(playlist|album|track)/([a-zA-Z0-9]+)`)
)

// Normalize normalise un texte pour la correspondance floue : les segments
// entre parenthèses/crochets/accollades sont retirés (« (Remastered) »…),
// la ponctuation supprimée, puis minuscules et espaces rognés.
func Normalize(text string) string {
	text = reBracket.ReplaceAllString(text, "")
	text = reNonWord.ReplaceAllString(text, "")
	return strings.TrimSpace(strings.ToLower(text))
}

// ExistingSet collecte tous les noms de fichiers .mp3 du dossier (récursif)
// sous forme de set de noms normalisés. Les fichiers « Title.mp3.mp3 » de
// spotdl perdent jusqu'à 2 extensions pour que le nom normalisé du fichier
// = le nom normalisé du titre. Lookup O(1) par morceau.
func ExistingSet(dir string) (map[string]struct{}, error) {
	existing := map[string]struct{}{}
	if _, err := os.Stat(dir); err != nil {
		// Dossier absent (ou illisible) : set vide, comme glob.glob Python.
		return existing, nil
	}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // fichier illisible : on l'ignore, on continue
		}
		if d.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".mp3") {
			return nil
		}
		base := filepath.Base(path)
		for i := 0; i < 2; i++ {
			base = strings.TrimSuffix(base, filepath.Ext(base))
		}
		if n := Normalize(base); n != "" {
			existing[n] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return existing, nil
	}
	return existing, nil
}

// Track est un morceau extrait de la page embed (forme JSON identique au
// script Python : title, artist, query, uri).
type Track struct {
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Query  string `json:"query"`
	URI    string `json:"uri"`
}

// equalTracks compare deux pages : l'API embed renvoie la MÊME page quand
// elle ignore l'offset — on le détecte pour ne pas compter les mêmes
// morceaux en boucle.
func equalTracks(a, b []Track) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// parseTrackList extrait la trackList de la page embed (playlists/albums).
func parseTrackList(html []byte) []Track {
	m := reNextData.FindSubmatch(html)
	if m == nil {
		return nil
	}
	var data struct {
		Props struct {
			PageProps struct {
				State struct {
					Data struct {
						Entity struct {
							TrackList []struct {
								Title    string `json:"title"`
								Subtitle string `json:"subtitle"`
								Artists  string `json:"artists"`
								URI      string `json:"uri"`
							} `json:"trackList"`
						} `json:"entity"`
					} `json:"data"`
				} `json:"state"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal(m[1], &data); err != nil {
		return nil
	}
	var out []Track
	for _, t := range data.Props.PageProps.State.Data.Entity.TrackList {
		title := strings.TrimSpace(t.Title)
		artists := strings.TrimSpace(t.Subtitle)
		if artists == "" {
			artists = strings.TrimSpace(t.Artists)
		}
		if title == "" {
			continue
		}
		q := title
		if artists != "" {
			q = artists + " - " + title
		}
		out = append(out, Track{Title: title, Artist: artists, Query: q, URI: t.URI})
	}
	return out
}

// parseTrackEntity construit la liste d'un morceau seul depuis l'entité de
// la page (la page d'un track n'a pas de trackList).
func parseTrackEntity(html []byte) []Track {
	m := reNextData.FindSubmatch(html)
	if m == nil {
		return nil
	}
	var data struct {
		Props struct {
			PageProps struct {
				State struct {
					Data struct {
						Entity struct {
							Title   string `json:"title"`
							Artists []struct {
								Name string `json:"name"`
							} `json:"artists"`
							URI string `json:"uri"`
						} `json:"entity"`
					} `json:"data"`
				} `json:"state"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal(m[1], &data); err != nil {
		return nil
	}
	ent := data.Props.PageProps.State.Data.Entity
	title := strings.TrimSpace(ent.Title)
	if title == "" {
		return nil
	}
	names := make([]string, 0, len(ent.Artists))
	for _, a := range ent.Artists {
		if strings.TrimSpace(a.Name) != "" {
			names = append(names, strings.TrimSpace(a.Name))
		}
	}
	artist := strings.Join(names, ", ")
	q := title
	if artist != "" {
		q = artist + " - " + title
	}
	return []Track{{Title: title, Artist: artist, Query: q, URI: ent.URI}}
}

// embedBaseURL construit l'endpoint embed depuis le lien collé lui-même,
// sans jamais hardcoder un domaine de fournisseur. Retourne "" si aucun
// hôte utilisable.
func embedBaseURL(mediaURL string) string {
	candidate := mediaURL
	if !strings.Contains(candidate, "://") {
		candidate = "https://" + candidate
	}
	u, err := url.Parse(candidate)
	if err != nil || u.Host == "" {
		return ""
	}
	host := u.Host
	if !strings.HasPrefix(host, "open.") {
		host = "open." + host
	}
	return "https://" + host + "/embed"
}

// FetchFunc récupère le contenu brut d'une URL (injectable dans les tests).
type FetchFunc func(url string) ([]byte, error)

// httpFetch est le fetch par défaut : en-tête navigateur, timeout 10 s,
// 3 tentatives espacées de 0,5 s (mêmes valeurs que le script Python).
func httpFetch(rawURL string) ([]byte, error) {
	var lastErr error
	client := &http.Client{Timeout: 10 * time.Second}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest(http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", userAgent)
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		return body, nil
	}
	return nil, lastErr
}

// FetchAllTracks pagine l'API embed pour récupérer TOUS les morceaux d'un
// lien playlist/album/track. Retourne truncated=true quand la liste est
// plafonnée (page répétée — l'API ignore l'offset — ou plus de titres que
// MAX_PAGES×PageSize) : dans ce cas l'app ne doit pas montrer de chiffres
// faux et laisse le moteur faire le vrai travail.
func FetchAllTracks(mediaURL string, fetch FetchFunc) (tracks []Track, truncated bool, err error) {
	if fetch == nil {
		fetch = httpFetch
	}
	m := reMedia.FindStringSubmatch(mediaURL)
	if m == nil {
		return nil, false, nil
	}
	base := embedBaseURL(mediaURL)
	if base == "" {
		return nil, false, nil
	}
	mediaType, mediaID := m[1], m[2]

	// Morceau seul : pas de pagination. La page n'a pas de trackList : on
	// retombe sur l'entité elle-même.
	if mediaType == "track" {
		page, ferr := fetch(base + "/" + mediaType + "/" + mediaID)
		if ferr != nil || len(page) == 0 {
			return nil, false, nil
		}
		if tr := parseTrackList(page); len(tr) > 0 {
			return tr, false, nil
		}
		return parseTrackEntity(page), false, nil
	}

	// Playlist/album : les pages en parallèle (l'API embed renvoie ~100
	// titres par page ; une page courte ou vide marque la fin de la liste).
	offsets := make([]int, 0, MaxPages)
	for off := 0; off < PageSize*MaxPages; off += PageSize {
		offsets = append(offsets, off)
	}
	type pageResult struct {
		off    int
		tracks []Track
	}
	results := make([]pageResult, len(offsets))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxWorkers)
	for i, off := range offsets {
		wg.Add(1)
		go func(i, off int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			pageURL := fmt.Sprintf("%s/%s/%s?offset=%d", base, mediaType, mediaID, off)
			page, ferr := fetch(pageURL)
			if ferr != nil {
				return // page vide : le tri final la fera interrompre
			}
			results[i] = pageResult{off: off, tracks: parseTrackList(page)}
		}(i, off)
	}
	wg.Wait()
	sort.Slice(results, func(i, j int) bool { return results[i].off < results[j].off })

	var all []Track
	var prev []Track
	broke := false
	for _, pr := range results {
		if len(pr.tracks) == 0 {
			broke = true
			break
		}
		if prev != nil && equalTracks(pr.tracks, prev) {
			// Même page renvoyée deux fois : offset ignoré, liste plafonnée.
			truncated = true
			broke = true
			break
		}
		all = append(all, pr.tracks...)
		prev = pr.tracks
		if len(pr.tracks) < PageSize {
			broke = true
			break
		}
	}
	if !broke {
		// Toutes les pages consommées sans page courte : plus de titres
		// (ou offset ignoré) — liste plafonnée.
		truncated = true
	}

	// Dédoublonnage : les pages peuvent se chevaucher ou se répéter.
	seen := map[string]struct{}{}
	var unique []Track
	for _, t := range all {
		k := strings.ToLower(strings.TrimSpace(t.Title)) + "\x00" + strings.ToLower(strings.TrimSpace(t.Artist))
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		unique = append(unique, t)
	}
	return unique, truncated, nil
}

// Filter compare les morceaux au set des noms de fichiers normalisés
// (lookup O(1) par morceau) et renvoie les requêtes déjà sur disque
// (skipped) et celles à télécharger (missing).
func Filter(existing map[string]struct{}, tracks []Track) (skipped, missing []string) {
	for _, t := range tracks {
		normTitle := Normalize(t.Title)
		normQuery := Normalize(t.Query)
		_, titleOK := existing[normTitle]
		_, queryOK := existing[normQuery]
		if (normTitle != "" && titleOK) || (normQuery != "" && queryOK) {
			skipped = append(skipped, t.Query)
		} else {
			missing = append(missing, t.Query)
		}
	}
	return skipped, missing
}

// Result est la forme JSON du résultat — mêmes clés que fast_filter.py
// (fast_filter_applied, reason, total_tracks, already_downloaded_count,
// missing_count, skipped_tracks, missing_queries).
type Result struct {
	Applied           bool     `json:"fast_filter_applied"`
	Reason            string   `json:"reason,omitempty"`
	TotalTracks       int      `json:"total_tracks"`
	AlreadyDownloaded int      `json:"already_downloaded_count"`
	MissingCount      int      `json:"missing_count"`
	SkippedTracks     []string `json:"skipped_tracks,omitempty"`
	MissingQueries    []string `json:"missing_queries,omitempty"`
}

// Run exécute le pipeline complet : set des fichiers sur disque + pagination
// de l'API embed + comparaison. Un fetch défaillant ne renvoie pas d'erreur
// (comportement du script Python) : le filtre est simplement désactivé avec
// une raison, et le moteur de téléchargement prend le relais.
func Run(dir, mediaURL string, fetch FetchFunc) Result {
	existing, _ := ExistingSet(dir)
	tracks, truncated, _ := FetchAllTracks(mediaURL, fetch)
	if len(tracks) == 0 {
		return Result{Applied: false, Reason: "No tracks extracted via embed API"}
	}
	if truncated {
		// La liste est plafonnée (~100 titres) : des chiffres « déjà sur
		// disque » seraient faux. On désactive le filtre — le moteur sait
		// résoudre la playlist complète et ignorer l'existant.
		return Result{
			Applied:     false,
			Reason:      "Spotify embed limité à ~100 titres — filtrage désactivé, spotdl gère le reste",
			TotalTracks: len(tracks),
		}
	}
	skipped, missing := Filter(existing, tracks)
	return Result{
		Applied:           true,
		TotalTracks:       len(tracks),
		AlreadyDownloaded: len(skipped),
		MissingCount:      len(missing),
		SkippedTracks:     skipped,
		MissingQueries:    missing,
	}
}
