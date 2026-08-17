package tags

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// ── Fixture : un vrai MP3 avec un tag ID3v2.3 complet ────────────────────
// Frames couvertes : texte (TIT2/TPE1/TALB/TPE2/TDRC/TCON/TRCK/TPOS/TEXT/
// TSRC/TCOP/TPUB), URL (WOAS), TXXX, COMM, people (TIPL/TMCL), APIC, puis
// des frames MPEG-1 Layer III (128 kbps, 44,1 kHz) pour durée + débit.

const (
	fixtureWOAS = "https://open.spotify.com/track/abc123"
	jpegCover   = "\xff\xd8\xff\xe0JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00\xff\xd9"
)

type tagFrame struct {
	id   string
	body []byte
}

func textFrame(id, text string) tagFrame {
	return tagFrame{id, append([]byte{0}, []byte(text)...)} // encodage 0 = latin-1
}

func urlFrame(id, url string) tagFrame {
	return tagFrame{id, []byte(url)} // les frames W n'ont pas d'octet d'encodage
}

func descFrame(id, desc, value string) tagFrame {
	return tagFrame{id, append(append([]byte{0}, []byte(desc)...), append([]byte{0}, []byte(value)...)...)}
}

func commFrame(lang, desc, text string) tagFrame {
	body := append([]byte{0}, []byte(lang)...) // enc + lang sur 3 octets
	body = append(body, []byte(desc)...)
	body = append(body, 0)
	return tagFrame{"COMM", append(body, []byte(text)...)}
}

func peopleFrame(id string, pairs ...string) tagFrame {
	body := []byte{0}
	body = append(body, []byte(strings.Join(pairs, "\x00"))...)
	return tagFrame{id, body}
}

func apicFrame(mime string, data []byte) tagFrame {
	body := append([]byte{0}, []byte(mime)...)
	body = append(body, 0, 3, 0) // type 3 = pochette, description vide
	return tagFrame{"APIC", append(body, data...)}
}

// buildTaggedMP3 assemble les frames ID3 + les frames MPEG et écrit le
// fichier sous downloadDir/<relPath>.
func buildTaggedMP3(t *testing.T, dir, relPath string, frames []tagFrame, audioFrames int) string {
	t.Helper()
	var body []byte
	for _, f := range frames {
		size := len(f.body)
		out := append([]byte(f.id),
			byte(size>>24), byte(size>>16), byte(size>>8), byte(size))
		out = append(out, 0, 0) // flags
		body = append(body, append(out, f.body...)...)
	}

	// Taille synchsafe (7 bits par octet).
	size := len(body)
	header := []byte{'I', 'D', '3', 3, 0, 0,
		byte(size >> 21 & 0x7F), byte(size >> 14 & 0x7F), byte(size >> 7 & 0x7F), byte(size & 0x7F)}

	// Frame MPEG-1 Layer III : 0xFF 0xFB 0x90 0x00 → 128 kbps, 44,1 kHz.
	frameLen := 144*128000/44100 - 4 // 417 - 4
	var audio []byte
	for i := 0; i < audioFrames; i++ {
		audio = append(audio, 0xFF, 0xFB, 0x90, 0x00)
		audio = append(audio, make([]byte, frameLen)...)
	}

	full := filepath.Join(dir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, append(append(header, body...), audio...), 0o644); err != nil {
		t.Fatalf("écriture du MP3 de test: %v", err)
	}
	return full
}

// fullFixture construit le MP3 avec toutes les frames du panneau détails.
func fullFixture(t *testing.T, dir, relPath string) string {
	t.Helper()
	frames := []tagFrame{
		textFrame("TIT2", "Airbag"),
		textFrame("TPE1", "Radiohead"),
		textFrame("TALB", "OK Computer"),
		textFrame("TPE2", "Radiohead"),
		textFrame("TDRC", "1997"),
		textFrame("TCON", "Alternative"),
		textFrame("TRCK", "5/12"),
		textFrame("TPOS", "1/1"),
		textFrame("TEXT", "Jonny Greenwood"),
		textFrame("TSRC", "GBARL0600242"),
		textFrame("TCOP", "1997 Parlophone"),
		textFrame("TPUB", "Parlophone"),
		urlFrame("WOAS", fixtureWOAS),
		commFrame("eng", "XXX", "Commentaire du morceau"),
		peopleFrame("TIPL", "producer", "Nigel Godrich", "engineer", "Jim Warren"),
		peopleFrame("TMCL", "drums", "Philip Selway"),
		descFrame("TXXX", "SONEPH_SOURCE", fixtureWOAS),
		descFrame("TXXX", "LYRICS_SOURCE", "musixmatch"),
		descFrame("TXXX", "SONEPH_QUALITY", "320kbps"),
		descFrame("TXXX", "LYRICS_SYNC_TYPE", "synced"),
		apicFrame("image/jpeg", []byte(jpegCover)),
	}
	return buildTaggedMP3(t, dir, relPath, frames, 40)
}

func TestMPEGDurationBitrate(t *testing.T) {
	path := buildTaggedMP3(t, t.TempDir(), "a.mp3", nil, 40)
	d, br := MPEGDurationBitrate(path)
	// 40 × 1152 échantillons / 44100 Hz ≈ 1044 ms.
	if d < 900 || d > 1200 {
		t.Errorf("durée = %d ms, want ~1044", d)
	}
	if br != 128 {
		t.Errorf("débit = %d kbps, want 128", br)
	}
}

func TestFileDetails(t *testing.T) {
	dir := t.TempDir()
	rel := "Radiohead/OK Computer/05-airbag.mp3"
	full := fullFixture(t, dir, rel)
	// Sidecar .lrc synchronisé.
	if err := os.WriteFile(filepath.Join(filepath.Dir(full), "05-airbag.lrc"),
		[]byte("[00:01.00]Some lyrics\n[00:05.50]More lyrics\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	d := FileDetails(full, rel)

	for key, want := range map[string]interface{}{
		"rel_path":         rel,
		"file_name":        "05-airbag.mp3",
		"title":            "Airbag",
		"artist":           "Radiohead",
		"album":            "OK Computer",
		"album_artist":     "Radiohead",
		"year":             "1997",
		"genre":            "Alternative",
		"track":            "5/12",
		"disc":             "1/1",
		"writer":           "Jonny Greenwood",
		"isrc":             "GBARL0600242",
		"copyright":        "1997 Parlophone",
		"publisher":        "Parlophone",
		"spotify_url":      fixtureWOAS,
		"comment":          "Commentaire du morceau",
		"bitrate":          "128kbps",
		"duration_seconds": 1,
		"lyrics_source":    "musixmatch",
		"quality":          "320kbps", // SONEPH_QUALITY prioritaire sur le bitrate
		"source_url":       fixtureWOAS,
		"lyrics_sync_type": "synced",
		"has_lyrics":       true,
		"lyrics_type":      "synced",
	} {
		if !reflect.DeepEqual(d[key], want) {
			t.Errorf("details[%q] = %#v, want %#v", key, d[key], want)
		}
	}

	people := d["involved_people"].([][]string)
	if !reflect.DeepEqual(people, [][]string{{"producer", "Nigel Godrich"}, {"engineer", "Jim Warren"}}) {
		t.Errorf("involved_people = %v", people)
	}
	musicians := d["musicians"].([][]string)
	if !reflect.DeepEqual(musicians, [][]string{{"drums", "Philip Selway"}}) {
		t.Errorf("musicians = %v", musicians)
	}

	custom, ok := d["custom_tags"].(map[string]string)
	if !ok || custom["SONEPH_SOURCE"] != fixtureWOAS || custom["LYRICS_SOURCE"] != "musixmatch" {
		t.Errorf("custom_tags = %#v", d["custom_tags"])
	}
	if _, ok := d["error"]; ok {
		t.Errorf("erreur inattendue : %v", d["error"])
	}
}

func TestFileDetailsNoTags(t *testing.T) {
	// MP3 sans tag ID3 : pas d'erreur, débit/durée quand même, lyrics none.
	dir := t.TempDir()
	full := buildTaggedMP3(t, dir, "a.mp3", nil, 40)
	d := FileDetails(full, "a.mp3")
	if _, ok := d["error"]; ok {
		t.Errorf("erreur inattendue : %v", d["error"])
	}
	if d["bitrate"] != "128kbps" {
		t.Errorf("bitrate = %v", d["bitrate"])
	}
	if d["has_lyrics"] != false || d["lyrics_type"] != "none" {
		t.Errorf("lyrics = has:%v type:%v, want false/none", d["has_lyrics"], d["lyrics_type"])
	}
	// Aucune clé « tags » : title absent, mais has_lyrics/lyrics_type présents.
	if _, ok := d["title"]; ok {
		t.Errorf("title présent sur un fichier sans tags")
	}
}

func TestFileDetailsMissing(t *testing.T) {
	d := FileDetails(filepath.Join(t.TempDir(), "nope.mp3"), "nope.mp3")
	if d["error"] != "file not found" {
		t.Errorf("error = %v, want file not found", d["error"])
	}
}

func TestIdentityMap(t *testing.T) {
	dir := t.TempDir()
	fullFixture(t, dir, "Radiohead/OK Computer/05-airbag.mp3")
	// Un MP3 sans WOAS ne doit pas apparaître.
	buildTaggedMP3(t, dir, "other/song.mp3", []tagFrame{textFrame("TIT2", "Sans URL")}, 10)

	m, err := IdentityMap(dir)
	if err != nil {
		t.Fatalf("IdentityMap: %v", err)
	}
	got := m["url:"+fixtureWOAS]
	want := []string{"Radiohead/OK Computer/05-airbag.mp3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("IdentityMap[%q] = %v, want %v", "url:"+fixtureWOAS, got, want)
	}
	if len(m) != 1 {
		t.Errorf("IdentityMap = %d entrées, want 1", len(m))
	}
}

func TestIdentityMapEmptyDir(t *testing.T) {
	m, err := IdentityMap(filepath.Join(t.TempDir(), "absent"))
	if err != nil {
		t.Fatalf("IdentityMap sur dossier absent: %v", err)
	}
	if len(m) != 0 {
		t.Errorf("carte non vide sur dossier absent : %v", m)
	}
}

func TestCover(t *testing.T) {
	full := fullFixture(t, t.TempDir(), "a.mp3")
	data, err := Cover(full)
	if err != nil {
		t.Fatalf("Cover: %v", err)
	}
	if string(data) != jpegCover {
		t.Errorf("cover = %d octets, want %d (contenu identique)", len(data), len(jpegCover))
	}
}

func TestCoverNone(t *testing.T) {
	full := buildTaggedMP3(t, t.TempDir(), "a.mp3", []tagFrame{textFrame("TIT2", "Sans pochette")}, 10)
	if _, err := Cover(full); err == nil {
		t.Error("Cover sur fichier sans APIC devrait échouer")
	}
}
