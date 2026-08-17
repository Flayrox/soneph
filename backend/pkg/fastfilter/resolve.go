package fastfilter

import (
	"encoding/json"
	"errors"
	"strings"
)

// ResolvePlaylist est le port Go de playlist_from_url.py : il résout les
// morceaux d'un lien (playlist / album / track) via l'API embed et les
// compare à la bibliothèque par IDENTITÉ — l'URL Spotify dans les tags WOAS
// (carte tags.IdentityMap) — et non par nom de fichier comme le fast filter.
// Il sert à créer une playlist « sans rien télécharger » à partir d'un lien
// dont on possède déjà les sons, et à compléter une playlist après un
// téléchargement.
//
// La correspondance est celle du script : uri spotify:track:<id> →
// identity["url:https://open.spotify.com/track/<id>"][0] → rel_path.
type PlaylistResolution struct {
	Name      string
	Matched   []PlaylistTrack
	Missing   []PlaylistTrack
	Total     int
	Truncated bool
}

// PlaylistTrack est un morceau résolu : title/artist/uri depuis l'embed,
// rel_path renseigné quand une copie existe déjà sur disque (matched).
type PlaylistTrack struct {
	Title   string `json:"title"`
	Artist  string `json:"artist"`
	URI     string `json:"uri"`
	RelPath string `json:"rel_path"`
}

// parseEntityName extrait le nom de la playlist/album depuis le
// __NEXT_DATA__ de la première page (name, sinon title — comme le script).
func parseEntityName(html []byte) string {
	m := reNextData.FindSubmatch(html)
	if m == nil {
		return ""
	}
	var data struct {
		Props struct {
			PageProps struct {
				State struct {
					Data struct {
						Entity struct {
							Name  string `json:"name"`
							Title string `json:"title"`
						} `json:"entity"`
					} `json:"data"`
				} `json:"state"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal(m[1], &data); err != nil {
		return ""
	}
	name := strings.TrimSpace(data.Props.PageProps.State.Data.Entity.Name)
	if name == "" {
		name = strings.TrimSpace(data.Props.PageProps.State.Data.Entity.Title)
	}
	return name
}

// trackIDFromURI extrait l'id d'un uri spotify:track:<id>.
func trackIDFromURI(uri string) (string, bool) {
	if !strings.HasPrefix(uri, "spotify:track:") {
		return "", false
	}
	id := strings.TrimPrefix(uri, "spotify:track:")
	if id == "" {
		return "", false
	}
	return id, true
}

// ResolvePlaylist pagine l'embed (FetchAllTracks), puis classe chaque
// morceau en matched (déjà sur disque, via la carte d'identité) ou missing.
// Un lien invalide ou une API indisponible renvoie une erreur — le handler
// laisse alors le téléchargement (spotdl) gérer le lien.
func ResolvePlaylist(mediaURL string, fetch FetchFunc, identity map[string][]string) (PlaylistResolution, error) {
	if fetch == nil {
		fetch = httpFetch
	}

	// Nom de la playlist/album depuis la première page. FetchAllTracks ne
	// renvoie pas le nom : on récupère la page 0 une fois de plus (un seul
	// GET de plus, au profit d'une signature de FetchAllTracks inchangée).
	name := ""
	if m := reMedia.FindStringSubmatch(mediaURL); m != nil {
		if base := embedBaseURL(mediaURL); base != "" {
			if page, err := fetch(base + "/" + m[1] + "/" + m[2]); err == nil {
				name = parseEntityName(page)
			}
		}
	}

	tracks, truncated, _ := FetchAllTracks(mediaURL, fetch)
	if len(tracks) == 0 {
		return PlaylistResolution{}, errors.New("Impossible de récupérer les morceaux du lien (lien invalide ou API indisponible)")
	}

	res := PlaylistResolution{Name: name, Total: len(tracks), Truncated: truncated}
	for _, t := range tracks {
		rel := ""
		if id, ok := trackIDFromURI(t.URI); ok {
			if paths := identity["url:https://open.spotify.com/track/"+id]; len(paths) > 0 {
				rel = paths[0]
			}
		}
		if rel != "" {
			res.Matched = append(res.Matched, PlaylistTrack{Title: t.Title, Artist: t.Artist, URI: t.URI, RelPath: rel})
		} else {
			res.Missing = append(res.Missing, PlaylistTrack{Title: t.Title, Artist: t.Artist, URI: t.URI})
		}
	}
	return res, nil
}
