package storage

import (
	"regexp"
	"sort"
	"strings"
)

// DuplicateGroup regroupe des fichiers considérés comme le même morceau :
// même titre + même artiste (normalisés), avec des variations de nom de
// fichier typiques d'Apple Music (« (1) », « copy », « 1-01 … », etc.).
type DuplicateGroup struct {
	Title       string           `json:"title"`
	Artist      string           `json:"artist"`
	Files       []DownloadedFile `json:"files"`
	KeepRelPath string           `json:"keep_rel_path"`
}

// accentReplacer retire les accents des titres/artistes pour rapprocher
// « Maman ne le sait pas » et « Maman ne le sait pas (1) ».
var accentReplacer = strings.NewReplacer(
	"é", "e", "è", "e", "ê", "e", "ë", "e", "à", "a", "â", "a", "ä", "a",
	"ù", "u", "û", "u", "ü", "u", "ç", "c", "î", "i", "ï", "i", "ô", "o",
	"ö", "o", "ÿ", "y", "ñ", "n",
	"É", "E", "È", "E", "Ê", "E", "Ë", "E", "À", "A", "Â", "A", "Ä", "A",
	"Ù", "U", "Û", "U", "Ü", "U", "Ç", "C", "Î", "I", "Ï", "I", "Ô", "O",
	"Ö", "O", "Ÿ", "Y", "Ñ", "N",
)

// reLeadingTrackNum retire les numéros de piste en tête de titre,
// typiques des rip CD : « 1-01 Titre », « 07 Titre », « 3. Titre ».
var reLeadingTrackNum = regexp.MustCompile(`^(?:[0-9]{1,2}-[0-9]{1,2}|[0-9]{1,2}[ ._])\s*`)

// reTrailingCopy retire les marqueurs de copie en fin de titre :
// « (1) », « copy », « - Copie », « (dupe) », « [2] »…
var reTrailingCopy = regexp.MustCompile(`(?i)(?:\s*[\[(]?[0-9]+[)\]]?\s*|\s*(?:copy|copie|dupe|duplicate)\s*)$`)

// normalizeTitle produit une clé stable et insensible aux variations :
// minuscules, sans accents ni ponctuation, numéro de piste et marqueur de
// copie retirés.
func normalizeTitle(s string) string {
	s = reLeadingTrackNum.ReplaceAllString(s, "")
	s = reTrailingCopy.ReplaceAllString(s, "")
	s = accentReplacer.Replace(s)
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// hasCopyMarker indique si le nom de fichier (brut) porte une marque de
// copie ou un numéro de piste — ces fichiers sont de moins bons candidats
// à garder.
func hasCopyMarker(raw string) bool {
	base := strings.TrimSuffix(raw, extensionOf(raw))
	if reLeadingTrackNum.MatchString(base) {
		return true
	}
	return reTrailingCopy.MatchString(accentReplacer.Replace(base))
}

func extensionOf(name string) string {
	i := strings.LastIndexByte(name, '.')
	if i < 0 {
		return ""
	}
	return name[i:]
}

// FindDuplicates regroupe les fichiers par (titre, artiste) normalisés et
// ne garde que les groupes d'au moins 2 fichiers. Le « gardé » conseillé
// est le fichier sans marqueur de copie (sinon le plus gros).
func FindDuplicates(files []DownloadedFile) []DuplicateGroup {
	groups := make(map[string]*DuplicateGroup)
	order := []string{}

	for i := range files {
		f := files[i]
		key := normalizeTitle(f.Title) + "\x00" + normalizeTitle(f.Artist)
		g, ok := groups[key]
		if !ok {
			g = &DuplicateGroup{Title: f.Title, Artist: f.Artist}
			groups[key] = g
			order = append(order, key)
		}
		g.Files = append(g.Files, f)
	}

	out := make([]DuplicateGroup, 0, len(order))
	for _, key := range order {
		g := groups[key]
		if len(g.Files) < 2 {
			continue
		}
		// Le meilleur candidat à garder : sans marqueur de copie, puis le
		// plus volumineux, puis le plus ancien (l'original).
		sort.SliceStable(g.Files, func(a, b int) bool {
			ma, mb := hasCopyMarker(g.Files[a].FileName), hasCopyMarker(g.Files[b].FileName)
			if ma != mb {
				return !ma
			}
			if g.Files[a].Size != g.Files[b].Size {
				return g.Files[a].Size > g.Files[b].Size
			}
			return g.Files[a].ModTime.Before(g.Files[b].ModTime)
		})
		g.KeepRelPath = g.Files[0].RelPath
		out = append(out, *g)
	}

	// Groupes triés par nombre de doublons décroissant (les plus flagrants
	// d'abord), puis par titre.
	sort.SliceStable(out, func(a, b int) bool {
		if len(out[a].Files) != len(out[b].Files) {
			return len(out[a].Files) > len(out[b].Files)
		}
		return strings.ToLower(out[a].Title) < strings.ToLower(out[b].Title)
	})
	return out
}
