package store

import (
	"os"
	"strings"

	"github.com/dhowden/tag"

	"soneph-backend/pkg/storage"
	"soneph-backend/pkg/tags"
)

// trackTagInfo est la partie des métadonnées lue depuis les tags du fichier
// (remplace file_details.py pour l'enrichissement de la base en M3/M5).
type trackTagInfo struct {
	TrackNo    int    // piste (TRCK / track number)
	DurationMs int    // durée, millisecondes
	Bitrate    int    // débit, kbps
	ISRC       string // code ISRC (frame TSRC en ID3v2, commentaire ISRC en Vorbis)
}

// readTags enrichit les infos d'un fichier scanné à partir de ses tags
// (github.com/dhowden/tag, lecture seule — §3). Pour le MP3, dhowden/tag ne
// donne pas la durée : on parse les frames MPEG (Layer I/II/III) nous-mêmes.
// Un fichier sans tags reconnaissables n'est pas une erreur : les champs
// restent à zéro.
func readTags(f storage.DownloadedFile) trackTagInfo {
	var info trackTagInfo

	// MPEG : durée + débit depuis les frames audio (indépendant des tags).
	// Le parseur vit dans pkg/tags (M6) — partagé avec FileDetails.
	if strings.HasSuffix(strings.ToLower(f.RelPath), ".mp3") {
		if d, br := tags.MPEGDurationBitrate(f.Path); d > 0 {
			info.DurationMs, info.Bitrate = d, br
		}
	}

	h, err := os.Open(f.Path)
	if err != nil {
		return info
	}
	defer h.Close()
	m, err := tag.ReadFrom(h)
	if err != nil {
		return info
	}

	if no, _ := m.Track(); no > 0 {
		info.TrackNo = no
	}

	// ISRC : frame TSRC en ID3v2, commentaire ISRC en Vorbis (FLAC/OGG).
	// (Cette version de dhowden/tag n'expose pas de durée : seul le parseur
	// MPEG ci-dessus la fournit, pour le MP3. FLAC/OGG/M4A auront une durée
	// avec le port M5 complet de l'écriture ID3v2.4.)
	if raw := m.Raw(); raw != nil {
		if v, ok := raw["TSRC"]; ok {
			info.ISRC = firstString(v)
		}
		if info.ISRC == "" {
			if v, ok := raw["ISRC"]; ok {
				info.ISRC = firstString(v)
			}
		}
	}
	return info
}

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
