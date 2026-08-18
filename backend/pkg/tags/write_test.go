package tags

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"
)

// ── Port de tag_soneph.py : marquage soneph (TXXX:SONEPH + source + qualité)

// mpegAudio construit N frames MPEG1 Layer III 128 kbps (44,1 kHz).
func mpegAudio(n int) []byte {
	frameLen := 144*128000/44100 - 4 // 417 - 4
	var audio []byte
	for i := 0; i < n; i++ {
		audio = append(audio, 0xFF, 0xFB, 0x90, 0x00)
		audio = append(audio, make([]byte, frameLen)...)
	}
	return audio
}

// writeMP3 écrit un fichier MP3 : tag ID3 (version, flags, frames) + audio.
func writeMP3(t *testing.T, version byte, flags byte, frames []tagFrame, audio []byte) string {
	t.Helper()
	var body []byte
	for _, f := range frames {
		if version == 2 {
			body = append(body, f.id[:3]...)
			body = append(body, byte(len(f.body)>>16), byte(len(f.body)>>8), byte(len(f.body)))
		} else if version == 4 {
			body = append(body, f.id...)
			body = append(body, syncsafe4(len(f.body))...)
			body = append(body, 0, 0)
		} else {
			body = append(body, f.id...)
			body = append(body, byte(len(f.body)>>24), byte(len(f.body)>>16), byte(len(f.body)>>8), byte(len(f.body)))
			body = append(body, 0, 0)
		}
		body = append(body, f.body...)
	}
	// Un tag « unsynchronisé » (flag 0x80) insère 0x00 après chaque 0xFF du
	// corps — les tailles de frames restent celles des données désyncées.
	onDisk := body
	if flags&0x80 != 0 {
		onDisk = make([]byte, 0, len(body))
		for i := 0; i < len(body); i++ {
			onDisk = append(onDisk, body[i])
			if body[i] == 0xFF {
				onDisk = append(onDisk, 0)
			}
		}
	}
	hdr := append([]byte{'I', 'D', '3', version, 0, flags}, syncsafe4(len(onDisk))...)
	path := filepath.Join(t.TempDir(), "f.mp3")
	if err := os.WriteFile(path, append(append(hdr, onDisk...), audio...), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// readFrames relit le tag ID3 du fichier (avec désunsync/ext header).
func readFrames(t *testing.T, path string) ([]id3Frame, byte) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	hdr := make([]byte, 10)
	if _, err := io.ReadFull(f, hdr); err != nil {
		t.Fatalf("lecture header: %v", err)
	}
	if string(hdr[0:3]) != "ID3" {
		t.Fatalf("pas de tag ID3 dans %s", path)
	}
	version := hdr[3]
	size := int(hdr[6])<<21 | int(hdr[7])<<14 | int(hdr[8])<<7 | int(hdr[9])
	body := make([]byte, size)
	if _, err := io.ReadFull(f, body); err != nil {
		t.Fatalf("lecture corps: %v", err)
	}
	if hdr[5]&0x80 != 0 {
		body = deunsync(body)
	}
	if hdr[5]&0x40 != 0 {
		var err error
		body, err = skipExtHeader(body, version)
		if err != nil {
			t.Fatalf("ext header: %v", err)
		}
	}
	frames, err := parseFrames(body, version)
	if err != nil {
		t.Fatalf("parseFrames: %v", err)
	}
	return frames, version
}

// txxxCount compte les frames TXXX/TXX dont la description vaut desc.
func txxxCount(frames []id3Frame, desc string) int {
	n := 0
	for _, f := range frames {
		if (f.id == "TXXX" || f.id == "TXX") && txxxDesc(f.body) == desc {
			n++
		}
	}
	return n
}

func TestStampSonephCreatesTag(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "no-tag.mp3"), mpegAudio(40), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := StampSoneph(dir, "https://open.spotify.com/playlist/xyz")
	if err != nil {
		t.Fatalf("StampSoneph: %v", err)
	}
	if n != 1 {
		t.Fatalf("fichiers marqués = %d, want 1", n)
	}

	frames, version := readFrames(t, filepath.Join(dir, "no-tag.mp3"))
	if version != 3 {
		t.Errorf("version = %d, want 3 (ID3v2.3 fraîche)", version)
	}
	if got := txxxCount(frames, "SONEPH"); got != 1 {
		t.Errorf("TXXX:SONEPH = %d, want 1", got)
	}
	if got := txxxCount(frames, "SONEPH_SOURCE"); got != 1 {
		t.Errorf("TXXX:SONEPH_SOURCE = %d, want 1", got)
	}
	if got := txxxCount(frames, "SONEPH_QUALITY"); got != 1 {
		t.Errorf("TXXX:SONEPH_QUALITY = %d, want 1", got)
	}

	// Valeurs lisibles par le reader (dhowden) — le panneau détails les lit.
	d := FileDetails(filepath.Join(dir, "no-tag.mp3"), "no-tag.mp3")
	custom, _ := d["custom_tags"].(map[string]string)
	if custom["SONEPH"] != "true" || custom["SONEPH_SOURCE"] != "https://open.spotify.com/playlist/xyz" {
		t.Errorf("custom_tags = %#v", custom)
	}
	if d["quality"] != "128kbps" || d["source_url"] != "https://open.spotify.com/playlist/xyz" {
		t.Errorf("quality=%v source_url=%v", d["quality"], d["source_url"])
	}
	// Les frames audio sont intactes (durée + débit relus).
	if d["bitrate"] != "128kbps" || d["duration_seconds"] != 1 {
		t.Errorf("bitrate=%v durée=%v", d["bitrate"], d["duration_seconds"])
	}
}

func TestStampSonephIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.mp3"), mpegAudio(40), 0o644); err != nil {
		t.Fatal(err)
	}
	url := "https://open.spotify.com/track/aaa"
	if n, err := StampSoneph(dir, url); err != nil || n != 1 {
		t.Fatalf("1er marquage: n=%d err=%v", n, err)
	}
	// Deuxième passage : rien ne change, aucune frame dupliquée.
	if n, err := StampSoneph(dir, url); err != nil || n != 0 {
		t.Fatalf("2e marquage: n=%d err=%v, want 0 (idempotent)", n, err)
	}
	frames, _ := readFrames(t, filepath.Join(dir, "a.mp3"))
	if got := txxxCount(frames, "SONEPH"); got != 1 {
		t.Errorf("TXXX:SONEPH = %d, want 1 (pas de doublon)", got)
	}
	if got := txxxCount(frames, "SONEPH_SOURCE"); got != 1 {
		t.Errorf("TXXX:SONEPH_SOURCE = %d, want 1", got)
	}
}

func TestStampSonephPreservesFrames(t *testing.T) {
	dir := t.TempDir()
	rel := "Radiohead/OK Computer/05-airbag.mp3"
	full := fullFixture(t, dir, rel) // tag v2.3 complet, QUALITY=320kbps
	url := "https://open.spotify.com/playlist/abc"
	if n, err := StampSoneph(dir, url); err != nil || n != 1 {
		t.Fatalf("StampSoneph: n=%d err=%v", n, err)
	}

	// Toutes les frames d'origine sont conservées ; SONEPH ajouté ; la
	// QUALITY « 320kbps » est remplacée par le débit réel (128 kbps) ;
	// SONEPH_SOURCE déjà présente n'est pas dupliquée.
	d := FileDetails(full, rel)
	for key, want := range map[string]interface{}{
		"title":         "Airbag",
		"artist":        "Radiohead",
		"album":         "OK Computer",
		"album_artist":  "Radiohead",
		"spotify_url":   fixtureWOAS,
		"comment":       "Commentaire du morceau",
		"lyrics_source": "musixmatch",
		"source_url":    fixtureWOAS, // SONEPH_SOURCE existante conservée
		"quality":       "128kbps",   // remplacée par le débit réel
	} {
		if !reflect.DeepEqual(d[key], want) {
			t.Errorf("details[%q] = %#v, want %#v", key, d[key], want)
		}
	}
	if got := d["involved_people"].([][]string); len(got) != 2 {
		t.Errorf("involved_people = %v, want 2 paires conservées", got)
	}
	if _, err := Cover(full); err != nil {
		t.Errorf("pochette APIC perdue : %v", err)
	}
	custom, _ := d["custom_tags"].(map[string]string)
	if custom["SONEPH"] != "true" {
		t.Errorf("custom_tags[SONEPH] = %q, want true", custom["SONEPH"])
	}

	frames, _ := readFrames(t, full)
	if got := txxxCount(frames, "SONEPH"); got != 1 {
		t.Errorf("TXXX:SONEPH = %d, want 1", got)
	}
	if got := txxxCount(frames, "SONEPH_SOURCE"); got != 1 {
		t.Errorf("TXXX:SONEPH_SOURCE = %d, want 1 (pas de doublon)", got)
	}
	if got := txxxCount(frames, "SONEPH_QUALITY"); got != 1 {
		t.Errorf("TXXX:SONEPH_QUALITY = %d, want 1 (remplacée, pas doublée)", got)
	}
}

func TestStampSonephQualityMatches(t *testing.T) {
	// QUALITY déjà au débit réel : aucune écriture (2e passage = 0).
	path := writeMP3(t, 3, 0, []tagFrame{descFrame("TXXX", "SONEPH_QUALITY", "128kbps")}, mpegAudio(40))
	dir := filepath.Dir(path)
	if n, err := StampSoneph(dir, ""); err != nil || n != 1 {
		t.Fatalf("marquage: n=%d err=%v, want 1 (SONEPH ajouté)", n, err)
	}
	if n, err := StampSoneph(dir, ""); err != nil || n != 0 {
		t.Fatalf("2e marquage: n=%d err=%v, want 0", n, err)
	}
	frames, _ := readFrames(t, path)
	if got := txxxCount(frames, "SONEPH_QUALITY"); got != 1 {
		t.Errorf("SONEPH_QUALITY = %d, want 1 (inchangée)", got)
	}
}

func TestStampSonephNoSource(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.mp3"), mpegAudio(40), 0o644); err != nil {
		t.Fatal(err)
	}
	if n, err := StampSoneph(dir, ""); err != nil || n != 1 {
		t.Fatalf("StampSoneph: n=%d err=%v", n, err)
	}
	frames, _ := readFrames(t, filepath.Join(dir, "a.mp3"))
	if got := txxxCount(frames, "SONEPH"); got != 1 {
		t.Errorf("TXXX:SONEPH = %d, want 1", got)
	}
	if got := txxxCount(frames, "SONEPH_SOURCE"); got != 0 {
		t.Errorf("TXXX:SONEPH_SOURCE = %d, want 0 (pas d'URL fournie)", got)
	}
}

func TestStampSonephV24(t *testing.T) {
	// Tag ID3v2.4 : la version est conservée, les frames préservées, et les
	// nouvelles frames écrites en tailles synchsafe.
	path := writeMP3(t, 4, 0, []tagFrame{
		textFrame("TIT2", "Airbag"),
		textFrame("TPE1", "Radiohead"),
		descFrame("TXXX", "LYRICS_SOURCE", "lrclib"),
	}, mpegAudio(40))
	if n, err := StampSoneph(filepath.Dir(path), "https://open.spotify.com/album/xyz"); err != nil || n != 1 {
		t.Fatalf("StampSoneph: n=%d err=%v", n, err)
	}
	frames, version := readFrames(t, path)
	if version != 4 {
		t.Errorf("version = %d, want 4 (conservée)", version)
	}
	if got := txxxCount(frames, "SONEPH"); got != 1 {
		t.Errorf("TXXX:SONEPH = %d, want 1", got)
	}
	d := FileDetails(path, "a.mp3")
	if d["title"] != "Airbag" || d["artist"] != "Radiohead" {
		t.Errorf("frames v2.4 perdues : title=%v artist=%v", d["title"], d["artist"])
	}
	if d["lyrics_source"] != "lrclib" {
		t.Errorf("lyrics_source = %v, want lrclib", d["lyrics_source"])
	}
	if d["source_url"] != "https://open.spotify.com/album/xyz" {
		t.Errorf("source_url = %v", d["source_url"])
	}
}

func TestStampSonephUnsynced(t *testing.T) {
	// Tag v2.3 avec drapeau d'unsynchronisation : les 0x00 insérés après
	// chaque 0xFF sont retirés avant le parse, frames préservées.
	body := append([]byte{0}, []byte("AB\xff\x00CD")...) // TIT2 latin-1 avec \xff
	frames := []tagFrame{{id: "TIT2", body: body}}
	path := writeMP3(t, 3, 0x80, frames, mpegAudio(40)) // flag unsync
	if n, err := StampSoneph(filepath.Dir(path), "src"); err != nil || n != 1 {
		t.Fatalf("StampSoneph: n=%d err=%v", n, err)
	}
	f, _ := readFrames(t, path)
	if got := txxxCount(f, "SONEPH"); got != 1 {
		t.Errorf("TXXX:SONEPH = %d, want 1", got)
	}
	d := FileDetails(path, "a.mp3")
	if d["title"] != "ABÿCD" { // 0xFF latin-1 = ÿ
		t.Errorf("title = %q, want ABÿCD (désunsynchronisé)", d["title"])
	}
}

func TestStampSonephPerms(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.mp3")
	if err := os.WriteFile(path, mpegAudio(40), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := StampSoneph(dir, "src"); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Errorf("permissions = %v, want 0600 (préservées)", st.Mode().Perm())
	}
}

// utf16leBOM encode en UTF-16 LE avec BOM — le format de chaque champ texte
// écrit par mutagen (encoding=1).
func utf16leBOM(s string) []byte {
	u := utf16.Encode([]rune(s))
	out := []byte{0xFF, 0xFE}
	for _, r := range u {
		out = append(out, byte(r), byte(r>>8))
	}
	return out
}

// mutagenTXXX construit une frame TXXX telle que mutagen la sérialise :
// encodage, description, séparateur (0x00 ou 0x00 0x00 en UTF-16), valeur,
// puis un séparateur de fin de liste après la dernière valeur.
func mutagenTXXX(enc byte, desc, value string) tagFrame {
	var body []byte
	if enc == 1 {
		body = append(body, 1)
		body = append(body, utf16leBOM(desc)...)
		body = append(body, 0, 0)
		body = append(body, utf16leBOM(value)...)
		body = append(body, 0, 0)
	} else {
		body = append(body, enc)
		body = append(body, desc...)
		body = append(body, 0)
		body = append(body, value...)
		body = append(body, 0)
	}
	return tagFrame{"TXXX", body}
}

// TestTXXXUTF16Alignment : les frames ré-écrites par mutagen (enc=1 UTF-16
// LE, BOM par champ) doivent être détectées avec leur description COMPLÈTE —
// un découpage des octets bruts sur 00 00 ronge le dernier caractère
// (bug réel : « SONEP » au lieu de « SONEPH »).
func TestTXXXUTF16Alignment(t *testing.T) {
	// Le corps exact d'une frame mutagen : 01 + BOM + "SONEPH" + 00 00 +
	// BOM + "true" + 00 00 (séparateur de fin de liste).
	body := append([]byte{1}, utf16leBOM("SONEPH")...)
	body = append(body, 0, 0)
	body = append(body, utf16leBOM("true")...)
	body = append(body, 0, 0)
	if got := txxxDesc(body); got != "SONEPH" {
		t.Errorf("desc = %q, want SONEPH (dernier caractère conservé)", got)
	}
	if got := txxxText(body); got != "true" {
		t.Errorf("text = %q, want true", got)
	}

	// Version UTF-16 BE (enc=2), sans BOM — même découpage sûr.
	var be []byte
	for _, r := range "SONEPH_SOURCE" {
		be = append(be, byte(r>>8), byte(r))
	}
	be = append(be, 0, 0)
	for _, r := range "https://open.spotify.com/track/abc" {
		be = append(be, byte(r>>8), byte(r))
	}
	be = append(be, 0, 0)
	body2 := append([]byte{2}, be...)
	if got := txxxDesc(body2); got != "SONEPH_SOURCE" {
		t.Errorf("desc BE = %q, want SONEPH_SOURCE", got)
	}
	if got := txxxText(body2); got != "https://open.spotify.com/track/abc" {
		t.Errorf("text BE = %q", got)
	}
}

// TestStampSonephMutagenRoundTrip : embed_lyrics.py ré-écrit tout le tag
// avec mutagen (encoding 1, UTF-16 avec BOM par champ et séparateurs finaux)
// APRÈS notre marquage. Au passage suivant, StampSoneph doit détecter les
// frames existantes (description et valeur nettoyées) et ne rien ré-écrire.
func TestStampSonephMutagenRoundTrip(t *testing.T) {
	path := writeMP3(t, 3, 0, []tagFrame{
		mutagenTXXX(1, "SONEPH_QUALITY", "128kbps"),
		mutagenTXXX(1, "SONEPH_SOURCE", "https://open.spotify.com/track/abc"),
	}, mpegAudio(40))
	dir := filepath.Dir(path)

	// Débit réel = 128 kbps, QUALITY déjà à 128 : seule SONEPH est ajoutée.
	if n, err := StampSoneph(dir, "https://open.spotify.com/track/abc"); err != nil || n != 1 {
		t.Fatalf("1er marquage: n=%d err=%v, want 1 (SONEPH ajouté, QUALITY conservée)", n, err)
	}
	// Idempotent : la QUALITY écrite par mutagen (enc=1) est reconnue.
	if n, err := StampSoneph(dir, "https://open.spotify.com/track/abc"); err != nil || n != 0 {
		t.Fatalf("2e marquage: n=%d err=%v, want 0 (frames mutagen reconnues)", n, err)
	}
	frames, _ := readFrames(t, path)
	if got := txxxCount(frames, "SONEPH_QUALITY"); got != 1 {
		t.Errorf("SONEPH_QUALITY = %d, want 1 (pas de doublon après round-trip mutagen)", got)
	}
	if got := txxxCount(frames, "SONEPH_SOURCE"); got != 1 {
		t.Errorf("SONEPH_SOURCE = %d, want 1", got)
	}

	// Version enc=3 (le format de tag_soneph.py) avec séparateur final.
	path2 := writeMP3(t, 3, 0, []tagFrame{mutagenTXXX(3, "SONEPH_QUALITY", "128kbps")}, mpegAudio(40))
	if n, err := StampSoneph(filepath.Dir(path2), ""); err != nil || n != 1 {
		t.Fatalf("enc=3: n=%d err=%v, want 1 (SONEPH ajouté)", n, err)
	}
	if n, err := StampSoneph(filepath.Dir(path2), ""); err != nil || n != 0 {
		t.Fatalf("enc=3 idempotent: n=%d err=%v, want 0", n, err)
	}
}

// TestFileDetailsMutagenFrames : les valeurs écrites par mutagen (enc=1,
// séparateur final) sont rendues propres par FileDetails — sans NUL final.
func TestFileDetailsMutagenFrames(t *testing.T) {
	path := writeMP3(t, 3, 0, []tagFrame{
		mutagenTXXX(1, "SONEPH_QUALITY", "320kbps"),
		mutagenTXXX(1, "SONEPH_SOURCE", "https://open.spotify.com/track/abc"),
	}, mpegAudio(40))
	d := FileDetails(path, "a.mp3")
	if d["quality"] != "320kbps" {
		t.Errorf("quality = %q, want 320kbps (sans NUL)", d["quality"])
	}
	if d["source_url"] != "https://open.spotify.com/track/abc" {
		t.Errorf("source_url = %q, want URL propre", d["source_url"])
	}
	custom, _ := d["custom_tags"].(map[string]string)
	if custom["SONEPH_QUALITY"] != "320kbps" || custom["SONEPH_SOURCE"] != "https://open.spotify.com/track/abc" {
		t.Errorf("custom_tags = %#v", custom)
	}
}

// TestStampSonephDedupesQuality : deux frames SONEPH_QUALITY cohabitent
// (ex. un ancien marquage à 319kbps à côté d'une frame mutagen à 320kbps) →
// le marquage n'en laisse qu'une, à la valeur du débit réel.
func TestStampSonephDedupesQuality(t *testing.T) {
	path := writeMP3(t, 3, 0, []tagFrame{
		descFrame("TXXX", "SONEPH_QUALITY", "320kbps"),
		descFrame("TXXX", "SONEPH_QUALITY", "319kbps"),
	}, mpegAudio(40))
	dir := filepath.Dir(path)
	if n, err := StampSoneph(dir, ""); err != nil || n != 1 {
		t.Fatalf("marquage: n=%d err=%v, want 1 (dédoublonnage)", n, err)
	}
	frames, _ := readFrames(t, path)
	if got := txxxCount(frames, "SONEPH_QUALITY"); got != 1 {
		t.Errorf("SONEPH_QUALITY = %d, want 1 (dédoublonnée)", got)
	}
	// La valeur restante est celle du débit réel.
	d := FileDetails(path, "a.mp3")
	if d["quality"] != "128kbps" {
		t.Errorf("quality = %v, want 128kbps (débit réel)", d["quality"])
	}
}

func TestStampSonephRecursiveAndNonMP3(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	paths := []string{
		"a.mp3",
		"sub/b.mp3",
		"sub/deep/c.MP3", // extension en majuscules
		"sub/cover.jpg",  // pas un MP3
		"sub/notes.txt",
	}
	for _, p := range paths {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(p)), mpegAudio(10), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	n, err := StampSoneph(dir, "src")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("fichiers marqués = %d, want 3 (les MP3 récursifs seulement)", n)
	}
	// Les non-MP3 ne sont pas touchés.
	for _, p := range []string{"sub/cover.jpg", "sub/notes.txt"} {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(p)))
		if err != nil || len(data) == 0 || strings.HasPrefix(string(data), "ID3") {
			t.Errorf("%s a été modifié", p)
		}
	}
}
