// ── Écrivain ID3v2 (port de tag_soneph.py, M6 part 2) ─────────────────────
// dhowden/tag ne sait que lire : ce fichier ajoute la brique d'écriture
// nécessaire au marquage soneph, sans sous-processus Python.
//
// Stratégie : on préserve les frames existantes OCTET POUR OCTET (splice),
// on n'ajoute/remplace que les frames qu'on gère, et on réécrit le fichier
// de façon atomique (temp + rename) en conservant la queue audio. C'est
// plus fidèle qu'une ré-écriture complète (aucune frame inconnue perdue) et
// la version du tag existant est conservée (ID3v2.3 fraîche si absent).

package tags

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// id3Frame est une frame ID3 avec son corps (taille et flags recalculés à
// la sérialisation, selon la version du tag).
type id3Frame struct {
	id   string
	body []byte
}

// StampSoneph marque chaque MP3 sous dir (récursif) avec les tags
// d'identité soneph — le port Go de tag_soneph.py :
//
//	TXXX:SONEPH         = "true"     (ajouté seulement si absent, idempotent)
//	TXXX:SONEPH_SOURCE  = sourceURL  (idem, si une URL est fournie)
//	TXXX:SONEPH_QUALITY = "<débit réel>kbps" (toujours rafraîchi si différent)
//
// Retourne le nombre de fichiers réellement modifiés. Un fichier illisible
// ou corrompu est ignoré silencieusement (comme le script, qui continue sur
// l'exception suivante).
func StampSoneph(dir, sourceURL string) (int, error) {
	n := 0
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".mp3") {
			return nil
		}
		if changed, serr := stampFile(path, sourceURL); serr == nil && changed {
			n++
		}
		return nil
	})
	return n, err
}

// stampFile marque un seul fichier. (false, nil) = rien à faire.
func stampFile(path, sourceURL string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	hdr := make([]byte, 10)
	if _, err := io.ReadFull(f, hdr); err != nil {
		return false, err // trop court : on laisse le fichier intact
	}

	// Pas de tag ID3 : on en crée un neuf en ID3v2.3 (comme mutagen ID3()).
	if string(hdr[0:3]) != "ID3" {
		_, frames := planFrames(nil, 3, sourceURL, qualityLabel(path))
		return writeWithTag(path, f, 0, 3, false, frames)
	}

	version := hdr[3]
	if version < 2 || version > 4 {
		return false, errors.New("version ID3 inconnue")
	}
	flags := hdr[5]
	size := int(hdr[6])<<21 | int(hdr[7])<<14 | int(hdr[8])<<7 | int(hdr[9])
	body := make([]byte, size)
	if _, err := io.ReadFull(f, body); err != nil {
		return false, err // taille déclarée plus grande que le fichier
	}
	footer := version == 4 && flags&0x10 != 0
	tailOffset := int64(10 + size)
	if footer {
		tailOffset += 10
	}

	data := body
	if flags&0x80 != 0 { // unsynchronisation : 0x00 insérés après chaque 0xFF
		data = deunsync(body)
	}
	if flags&0x40 != 0 { // extended header
		data, err = skipExtHeader(data, version)
		if err != nil {
			return false, err
		}
	}
	frames, err := parseFrames(data, version)
	if err != nil {
		return false, err
	}

	changed, out := planFrames(frames, version, sourceURL, qualityLabel(path))
	if !changed {
		return false, nil
	}
	return writeWithTag(path, f, tailOffset, version, footer, out)
}

// qualityLabel calcule le libellé TXXX:SONEPH_QUALITY à partir du débit réel
// du fichier (débit moyen, parité mutagen) — vide si aucune frame valide.
func qualityLabel(path string) string {
	if br := AverageBitrateKbps(path); br > 0 {
		return strconv.Itoa(br) + "kbps"
	}
	return ""
}

// planFrames décide du nouvel ensemble de frames : préservation intégrale,
// ajout de SONEPH / SONEPH_SOURCE seulement si absents, remplacement de
// SONEPH_QUALITY seulement si le débit réel diffère (comme tag_soneph.py :
// pas d'écriture quand rien ne change).
func planFrames(frames []id3Frame, version byte, sourceURL, label string) (changed bool, out []id3Frame) {
	var quality []id3Frame
	hasSoneph, hasSource := false, false
	for _, f := range frames {
		if f.id == "TXXX" || f.id == "TXX" {
			switch txxxDesc(f.body) {
			case "SONEPH":
				hasSoneph = true
			case "SONEPH_SOURCE":
				hasSource = true
			case "SONEPH_QUALITY":
				quality = append(quality, f)
				continue
			}
		}
		out = append(out, f)
	}

	// Remplacement si le débit réel diffère, OU si plusieurs frames QUALITY
	// cohabitent (dédoublonnage — ex. un ancien marquage à 319kbps à côté
	// d'une frame mutagen à 320kbps).
	replaceQuality := label != "" && (len(quality) != 1 || txxxText(quality[0].body) != label)
	if !replaceQuality {
		out = append(out, quality...)
	}
	if !hasSoneph {
		out = append(out, makeUserText(version, "SONEPH", "true"))
		changed = true
	}
	if sourceURL != "" && !hasSource {
		out = append(out, makeUserText(version, "SONEPH_SOURCE", sourceURL))
		changed = true
	}
	if replaceQuality {
		out = append(out, makeUserText(version, "SONEPH_QUALITY", label))
		changed = true
	}
	return changed, out
}

// makeUserText construit une frame TXXX (TXX en ID3v2.2) avec l'encodage 3
// (UTF-8), comme mutagen encoding=3 — lu par dhowden et mutagen ; les
// valeurs écrites (URL, "true", "320kbps") sont de l'ASCII.
func makeUserText(version byte, desc, value string) id3Frame {
	id := "TXXX"
	if version == 2 {
		id = "TXX"
	}
	body := []byte{3}
	body = append(body, desc...)
	body = append(body, 0)
	body = append(body, value...)
	return id3Frame{id, body}
}

// txxxDesc extrait la description d'une frame TXXX/TXX (octet d'encodage +
// description + séparateur), pour tous les encodages (0 latin-1, 1 UTF-16
// avec BOM, 2 UTF-16 BE, 3 UTF-8) — la détection des frames SONEPH déjà
// présentes fonctionne quelle que soit la façon dont un autre outil les a
// écrites (mutagen, spotdl…).
func txxxDesc(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	enc := body[0]
	rest := body[1:]
	if enc == 1 || enc == 2 {
		// Découpage APRÈS décodage UTF-16 : couper les octets bruts sur
		// 00 00 décale d'un octet en UTF-16LE (le zéro bas du dernier
		// caractère précède le séparateur) et ronge le dernier caractère
		// de la description (bug réel : « SONEP » au lieu de « SONEPH »
		// sur les frames ré-écrites par mutagen).
		s := decodeUTF16(rest)
		if i := strings.IndexByte(s, 0); i >= 0 {
			return trimTagField(s[:i])
		}
		return trimTagField(s)
	}
	if i := bytes.IndexByte(rest, 0); i >= 0 {
		return trimTagField(string(rest[:i]))
	}
	return trimTagField(string(rest))
}

// txxxText extrait la valeur d'une frame TXXX/TXX (après la description).
func txxxText(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	enc := body[0]
	rest := body[1:]
	if enc == 1 || enc == 2 {
		s := decodeUTF16(rest)
		if i := strings.IndexByte(s, 0); i >= 0 {
			return trimTagField(s[i+1:])
		}
		return ""
	}
	if i := bytes.IndexByte(rest, 0); i >= 0 {
		return trimTagField(string(rest[i+1:]))
	}
	return ""
}

// trimTagField normalise un champ texte TXXX quelle que soit la façon dont
// mutagen l'a écrit : séparateurs de fin de liste (\x00 en latin-1/UTF-8,
// \x00\x00 en UTF-16 — mutagen en ajoute un après la dernière valeur) et
// BOM de début. Sans cela, la comparaison du débit (idempotence QUALITY)
// échouerait après un aller-retour mutagen (embed_lyrics.py ré-écrit le tag).
func trimTagField(s string) string {
	return strings.TrimRight(strings.TrimPrefix(s, "\uFEFF"), "\x00")
}

// parseFrames décode la liste de frames d'un tag ID3v2 (v2.2 : en-têtes de
// 6 octets id3+taille3 ; v2.3/v2.4 : 10 octets id4+taille4+flags2, taille
// synchsafe en v2.4). Le padding (0x00) met fin au parcours.
func parseFrames(data []byte, version byte) ([]id3Frame, error) {
	var frames []id3Frame
	for len(data) > 0 {
		if data[0] == 0 {
			break // padding
		}
		if version == 2 {
			if len(data) < 6 {
				return nil, errors.New("frame ID3v2.2 tronquée")
			}
			id := string(data[0:3])
			flen := int(data[3])<<16 | int(data[4])<<8 | int(data[5])
			data = data[6:]
			if flen > len(data) {
				return nil, errors.New("frame ID3v2.2 dépassée")
			}
			frames = append(frames, id3Frame{id, data[:flen]})
			data = data[flen:]
			continue
		}
		if len(data) < 10 {
			return nil, errors.New("frame ID3 tronquée")
		}
		id := string(data[0:4])
		var flen int
		if version == 4 {
			flen = int(data[4])<<21 | int(data[5])<<14 | int(data[6])<<7 | int(data[7])
		} else {
			flen = int(data[4])<<24 | int(data[5])<<16 | int(data[6])<<8 | int(data[7])
		}
		data = data[10:]
		if flen > len(data) {
			return nil, errors.New("frame ID3 dépassée")
		}
		frames = append(frames, id3Frame{id, data[:flen]})
		data = data[flen:]
	}
	return frames, nil
}

// serializeFrames écrit les frames avec le format de taille propre à la
// version (3 octets en v2.2, 4 octets en v2.3, synchsafe en v2.4).
func serializeFrames(frames []id3Frame, version byte) []byte {
	var out bytes.Buffer
	for _, f := range frames {
		id := f.id
		n := len(f.body)
		if version == 2 {
			out.Write([]byte{id[0], id[1], id[2], byte(n >> 16), byte(n >> 8), byte(n)})
		} else {
			var sz []byte
			if version == 4 {
				sz = syncsafe4(n)
			} else {
				sz = []byte{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}
			}
			out.Write([]byte{id[0], id[1], id[2], id[3]})
			out.Write(sz)
			out.Write([]byte{0, 0}) // flags
		}
		out.Write(f.body)
	}
	return out.Bytes()
}

// writeWithTag réécrit le fichier : nouveau tag ID3 (header + frames +
// footer v2.4 éventuel) puis la queue audio d'origine, de façon atomique
// (temp dans le même dossier + rename, permissions préservées).
func writeWithTag(path string, src *os.File, tailOffset int64, version byte, footer bool, frames []id3Frame) (bool, error) {
	var buf bytes.Buffer
	frameData := serializeFrames(frames, version)
	size := len(frameData)

	header := []byte{'I', 'D', '3', version, 0, 0}
	header = append(header, syncsafe4(size)...)
	if footer {
		header[5] |= 0x10
	}
	buf.Write(header)
	buf.Write(frameData)
	if footer {
		buf.Write([]byte{'3', 'D', 'I'}) // footer : miroir du header
		buf.Write(syncsafe4(size))
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".soneph-stamp-*")
	if err != nil {
		return false, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // nettoyage si erreur avant le rename

	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		return false, err
	}
	if _, err := src.Seek(tailOffset, io.SeekStart); err != nil {
		tmp.Close()
		return false, err
	}
	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		return false, err
	}
	if err := tmp.Close(); err != nil {
		return false, err
	}
	if st, err := os.Stat(path); err == nil {
		_ = os.Chmod(tmpName, st.Mode().Perm())
	}
	return true, os.Rename(tmpName, path)
}

// syncsafe4 encode un entier sur 4 octets synchsafe (7 bits utiles chacun).
func syncsafe4(n int) []byte {
	return []byte{byte(n >> 21 & 0x7F), byte(n >> 14 & 0x7F), byte(n >> 7 & 0x7F), byte(n & 0x7F)}
}

// deunsync retire les 0x00 d'unsynchronisation (insérés après chaque 0xFF
// dans le corps du tag) — l'inverse de la transformation ID3 « unsync ».
func deunsync(b []byte) []byte {
	out := make([]byte, 0, len(b))
	for i := 0; i < len(b); i++ {
		out = append(out, b[i])
		if b[i] == 0xFF && i+1 < len(b) && b[i+1] == 0x00 {
			i++
		}
	}
	return out
}

// skipExtHeader saute l'extended header d'un tag ID3v2.3/v2.4 (taille non
// synchsafe en v2.3 — incluse dans le champ taille —, synchsafe en v2.4).
func skipExtHeader(data []byte, version byte) ([]byte, error) {
	if len(data) < 4 {
		return nil, errors.New("extended header tronqué")
	}
	if version == 3 {
		n := int(data[0])<<24 | int(data[1])<<16 | int(data[2])<<8 | int(data[3])
		if n < 4 || n > len(data) {
			return nil, errors.New("extended header invalide")
		}
		return data[n:], nil
	}
	n := int(data[0])<<21 | int(data[1])<<14 | int(data[2])<<7 | int(data[3])
	if n+4 > len(data) {
		return nil, errors.New("extended header invalide")
	}
	return data[4+n:], nil
}
