// Package tags lit les métadonnées ID3 d'un fichier audio en Go
// (github.com/dhowden/tag), sans dépendre des scripts Python ni de mutagen.
//
// Il porte les trois scripts de lecture remplacés en M6 :
//   - IdentityMap  ← scan_identity.py  (carte {URL Spotify → chemins})
//   - FileDetails  ← file_details.py   (dump complet des frames ID3)
//   - Cover        ← extract_cover.py  (pochette embarquée APIC)
//   - MPEGDurationBitrate ← parseur de frames MPEG (débit + durée), déplacé
//     depuis pkg/store/tags.go (utilisé par le store ET par FileDetails).
//
// Lecture seule, zéro CGO : dhowden/tag ne sait pas écrire, ce qui est
// exactement ce qu'il faut ici (les écritures restent au moteur spotdl,
// M6 part 2 documenté dans docs/adr/0004-python-strangulation.md).
package tags

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/dhowden/tag"
)

// firstString normalise une valeur Raw() (string ou []string) en string.
func firstString(v interface{}) string {
	switch s := v.(type) {
	case string:
		return strings.TrimSpace(s)
	case []string:
		if len(s) > 0 {
			return strings.TrimSpace(s[0])
		}
	}
	return ""
}

// IdentityMap construit la carte {identité → [rel_path, ...]} de la
// bibliothèque en lisant le tag WOAS (URL Spotify) de chaque MP3 — le port
// Go de scan_identity.py. L'identité stable d'un morceau est son URL
// Spotify : c'est elle que le moteur utilise pour retrouver un fichier et le
// déplacer vers son album quand il est re-téléchargé dans un autre contexte
// (single → album).
func IdentityMap(downloadDir string) (map[string][]string, error) {
	out := map[string][]string{}
	err := filepath.WalkDir(downloadDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // fichier illisible : on l'ignore, on continue
		}
		if d.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".mp3") {
			return nil
		}
		rel, rerr := filepath.Rel(downloadDir, path)
		if rerr != nil {
			return nil
		}
		h, herr := os.Open(path)
		if herr != nil {
			return nil
		}
		m, merr := tag.ReadFrom(h)
		h.Close()
		if merr != nil {
			return nil
		}
		if raw := m.Raw(); raw != nil {
			if v, ok := raw["WOAS"]; ok {
				if u := firstString(v); u != "" {
					out["url:"+u] = append(out["url:"+u], filepath.ToSlash(rel))
				}
			}
		}
		return nil
	})
	return out, err
}

// Cover extrait la pochette embarquée (frame APIC) d'un fichier audio — le
// port Go de extract_cover.py. Retourne les octets de l'image telle quelle.
func Cover(path string) ([]byte, error) {
	h, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer h.Close()
	m, err := tag.ReadFrom(h)
	if err != nil {
		return nil, err
	}
	p := m.Picture()
	if p == nil || len(p.Data) == 0 {
		return nil, errors.New("no embedded artwork")
	}
	return p.Data, nil
}

// frame lit une frame texte simple depuis Raw() (string ou []string).
func frame(raw map[string]interface{}, key string) interface{} {
	v, ok := raw[key]
	if !ok {
		return nil
	}
	if s := firstString(v); s != "" {
		return s
	}
	return nil
}

// commFrames renvoie toutes les frames de type *Comm (TXXX / COMM) dont la
// clé Raw() commence par prefix (« TXXX », « COMM »…) — dhowden suffixe les
// doublons (TXXX_0, TXXX_1…).
func commFrames(raw map[string]interface{}, prefix string) []*tag.Comm {
	var out []*tag.Comm
	for k, v := range raw {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		if c, ok := v.(*tag.Comm); ok {
			out = append(out, c)
		}
	}
	return out
}

// peopleFramesFromFile lit les frames « people » (TIPL/TMCL en ID3v2.4,
// IPLS en v2.3) directement depuis les octets du fichier — l'équivalent de
// la propriété .people de mutagen (paires rôle, nom). dhowden/tag aplatit
// ces frames (readTFrame retire les séparateurs \x00 des frames texte), on
// ne peut donc pas les récupérer via Raw().
func peopleFramesFromFile(path string) (involved, musicians [][]string) {
	f, err := os.Open(path)
	if err != nil {
		return [][]string{}, [][]string{}
	}
	defer f.Close()
	hdr := make([]byte, 10)
	if _, err := io.ReadFull(f, hdr); err != nil || string(hdr[0:3]) != "ID3" {
		return [][]string{}, [][]string{}
	}
	version := hdr[3]
	if version == 2 {
		return [][]string{}, [][]string{} // ID3v2.2 : IDs sur 3 octets, ignoré
	}
	flags := hdr[5]
	size := int(hdr[6])<<21 | int(hdr[7])<<14 | int(hdr[8])<<7 | int(hdr[9])

	if flags&0x40 != 0 { // extended header
		exthdr := make([]byte, 4)
		if _, err := io.ReadFull(f, exthdr); err != nil {
			return [][]string{}, [][]string{}
		}
		n := int(exthdr[0])<<21 | int(exthdr[1])<<14 | int(exthdr[2])<<7 | int(exthdr[3])
		if version == 3 {
			n += 4 // v2.3 : la taille exclut le champ taille lui-même
		}
		if _, err := f.Seek(int64(n), io.SeekCurrent); err != nil {
			return [][]string{}, [][]string{}
		}
	}

	remaining := size
	for remaining >= 10 {
		h := make([]byte, 10)
		if _, err := io.ReadFull(f, h); err != nil {
			break
		}
		remaining -= 10
		id := string(h[0:4])
		flen := int(h[4])<<24 | int(h[5])<<16 | int(h[6])<<8 | int(h[7])
		if flen <= 0 || flen > remaining {
			break
		}
		body := make([]byte, flen)
		if _, err := io.ReadFull(f, body); err != nil {
			break
		}
		remaining -= flen

		var pairs [][]string
		switch id {
		case "TIPL", "IPLS":
			pairs = parsePeopleBody(body)
			involved = append(involved, pairs...)
		case "TMCL":
			pairs = parsePeopleBody(body)
			musicians = append(musicians, pairs...)
		}
	}
	if involved == nil {
		involved = [][]string{}
	}
	if musicians == nil {
		musicians = [][]string{}
	}
	return involved, musicians
}

// parsePeopleBody décode le corps d'une frame people : octet d'encodage,
// puis paires rôle/nom séparées par \x00 (\x00\x00 en UTF-16).
func parsePeopleBody(body []byte) [][]string {
	if len(body) == 0 {
		return nil
	}
	var s string
	switch body[0] {
	case 0: // latin-1
		s = string(body[1:])
	case 1: // UTF-16 avec BOM
		if len(body) < 3 {
			return nil
		}
		s = decodeUTF16(body[1:])
	case 2: // UTF-16 sans BOM (big-endian)
		if len(body) < 3 {
			return nil
		}
		s = decodeUTF16BigEndian(body[1:])
	case 3: // UTF-8
		s = string(body[1:])
	default:
		return nil
	}
	parts := strings.Split(s, "\x00")
	var pairs [][]string
	for i := 0; i+1 < len(parts); i += 2 {
		role := strings.TrimSpace(parts[i])
		name := strings.TrimSpace(parts[i+1])
		if role == "" && name == "" {
			continue
		}
		pairs = append(pairs, []string{role, name})
	}
	return pairs
}

// decodeUTF16 décode de l'UTF-16 avec BOM ; decodeUTF16BigEndian sans BOM
// (encodages 1 et 2 de l'ID3v2, gros-boutiste par défaut).
func decodeUTF16(b []byte) string {
	if len(b) < 2 {
		return ""
	}
	if b[0] == 0xFF && b[1] == 0xFE {
		return decodeUTF16LE(b[2:])
	}
	if b[0] == 0xFE && b[1] == 0xFF {
		return decodeUTF16BE(b[2:])
	}
	return decodeUTF16BE(b)
}

func decodeUTF16BigEndian(b []byte) string {
	return decodeUTF16BE(b)
}

func decodeUTF16BE(b []byte) string {
	u := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		u = append(u, uint16(b[i])<<8|uint16(b[i+1]))
	}
	return string(utf16.Decode(u))
}

func decodeUTF16LE(b []byte) string {
	u := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		u = append(u, uint16(b[i])|uint16(b[i+1])<<8)
	}
	return string(utf16.Decode(u))
}

// reLyricsTimestamp détecte les paroles synchronisées dans un .lrc
// (mêmes marques [mm:ss.xx] / [mm:ss:xx] que file_details.py).
var reLyricsTimestamp = regexp.MustCompile(`\[\d{2}:\d{2}[.:]\d{2,3}\]`)

// FileDetails produit le dump complet des métadonnées ID3 d'un fichier —
// le port Go de file_details.py, avec exactement les mêmes clés JSON (le
// panneau « Plus de détails » du frontend lit quality, duration_seconds,
// lyrics_type, involved_people, musicians, source_url…). Sur fichier
// absent, renvoie {"error": "file not found"} (le handler mappe ce cas sur
// 404, comme avec le script).
func FileDetails(fullPath, relPath string) map[string]interface{} {
	details := map[string]interface{}{
		"rel_path":  relPath,
		"file_name": filepath.Base(fullPath),
	}
	if _, err := os.Stat(fullPath); err != nil {
		details["error"] = "file not found"
		return details
	}

	// MP3 : débit + durée depuis les frames audio (dhowden/tag ne donne pas
	// la durée pour le MP3). Autres formats : dhowden expose Length() pour
	// FLAC/OGG/M4A — le débit n'est pas fourni (on l'omet, comme Python
	// l'omet quand mutagen ne le connaît pas).
	if strings.EqualFold(filepath.Ext(fullPath), ".mp3") {
		if d, br := MPEGDurationBitrate(fullPath); d > 0 {
			details["bitrate"] = strconv.Itoa(br) + "kbps"
			details["duration_seconds"] = (d + 500) / 1000
		}
	}

	h, err := os.Open(fullPath)
	if err != nil {
		return details
	}
	m, err := tag.ReadFrom(h)
	h.Close()
	if err != nil {
		// Fichier sans tags ID3 lisibles : pas d'erreur (Python continue
		// aussi avec un ID3NoHeaderError), mais pas de clés « tags ».
		lyricsType(details, fullPath)
		return details
	}
	raw := m.Raw()

	if s := m.Title(); s != "" {
		details["title"] = s
	}
	if s := m.Artist(); s != "" {
		details["artist"] = s
	}
	if s := m.Album(); s != "" {
		details["album"] = s
	}
	if s := m.AlbumArtist(); s != "" {
		details["album_artist"] = s
	}
	for key, out := range map[string]string{
		"TDRC": "year", "TCON": "genre", "TRCK": "track", "TPOS": "disc",
		"TEXT": "writer", "TSRC": "isrc", "TCOP": "copyright",
	} {
		if v := frame(raw, key); v != nil {
			details[out] = v
		}
	}
	// ISRC : TSRC en ID3v2, commentaire ISRC en Vorbis.
	if details["isrc"] == nil {
		if v := frame(raw, "ISRC"); v != nil {
			details["isrc"] = v
		}
	}
	if v := frame(raw, "TPUB"); v != nil {
		details["publisher"] = v
	} else if v := frame(raw, "TENC"); v != nil {
		details["publisher"] = v
	}
	if v := frame(raw, "WOAS"); v != nil {
		details["spotify_url"] = v
	}
	// Commentaire : frame COMM avec description « XXX » (COMM::XXX en
	// mutagen — la description par défaut écrite par le moteur).
	for _, c := range commFrames(raw, "COMM") {
		if c.Description == "XXX" && c.Text != "" {
			details["comment"] = c.Text
			break
		}
	}

	involved, musicians := peopleFramesFromFile(fullPath)
	details["involved_people"] = involved
	details["musicians"] = musicians

	custom := map[string]string{}
	for _, c := range commFrames(raw, "TXXX") {
		if c.Description == "" {
			continue
		}
		custom[c.Description] = c.Text
	}
	details["custom_tags"] = custom
	details["lyrics_source"] = custom["LYRICS_SOURCE"]
	if q, ok := custom["SONEPH_QUALITY"]; ok && q != "" {
		details["quality"] = q
	} else {
		details["quality"] = details["bitrate"]
	}
	details["source_url"] = custom["SONEPH_SOURCE"]
	details["lyrics_sync_type"] = custom["LYRICS_SYNC_TYPE"]

	lyricsType(details, fullPath)
	return details
}

// lyricsType renseigne has_lyrics / lyrics_type depuis le sidecar .lrc
// (synced si des horodatages [mm:ss.xx] y figurent, sinon unsynced).
func lyricsType(details map[string]interface{}, fullPath string) {
	ext := filepath.Ext(fullPath)
	lrcPath := strings.TrimSuffix(fullPath, ext) + ".lrc"
	details["has_lyrics"] = false
	details["lyrics_type"] = "none"
	content, err := os.ReadFile(lrcPath)
	if err != nil {
		return
	}
	details["has_lyrics"] = true
	if len(content) > 65536 {
		content = content[:65536]
	}
	if reLyricsTimestamp.Match(content) {
		details["lyrics_type"] = "synced"
	} else {
		details["lyrics_type"] = "unsynced"
	}
}
