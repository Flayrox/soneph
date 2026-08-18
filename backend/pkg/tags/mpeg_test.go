package tags

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

// ── Blindage du parseur MPEG : un scan de démarrage ne doit JAMAIS crasher ──
// Grille exhaustive sur tout l'espace d'en-têtes (version × layer × brIdx ×
// srIdx × padding = 4×4×16×4×2 = 2048 combinaisons), y compris les valeurs
// réservées/invalides, puis fichiers aléatoires et tronqués.

// mpegHeader assemble un en-tête MPEG de 4 octets depuis ses champs.
func mpegHeader(version, layer, brIdx, srIdx, padding int) []byte {
	b2 := byte(0xE0 | (version << 3) | (layer << 1)) // sync 11 bits + version + layer
	b3 := byte(brIdx<<4 | srIdx<<2 | padding<<1)
	return []byte{0xFF, b2, b3, 0x00}
}

// wantBitrate renvoie le débit attendu par les tables ISO 11172-3 pour une
// combinaison (version, layer, brIdx) — le miroir exact de la logique du
// parseur, pour vérifier que les en-têtes valides sont consommés avec la
// bonne valeur. Attention au codage du champ layer (spéc. MPEG) : 3 = Layer I,
// 2 = Layer II, 1 = Layer III — comme dans le parseur.
func wantBitrate(version, layer, brIdx int) int {
	if layer == 3 || version == 3 { // Layer I (toujours L3) et MPEG1 (L3 pour II/III)
		return mpegBitrateL3[brIdx]
	}
	return mpegBitrateL3Low[brIdx]
}

// validCombo : combinaison que le parseur doit accepter comme frame valide
// (version non réservée, layer non réservé, débit ≠ 0, échantillonnage ≠ 0).
func validCombo(version, layer, brIdx, srIdx int) bool {
	return version != 1 && layer != 0 && brIdx != 0 && brIdx != 15 && srIdx != 3
}

// mp3FrameLen reproduit la longueur de frame du parseur (pour construire un
// corps de frame complet dans les fixtures).
func mp3FrameLen(version, layer, brIdx, srIdx, padding int) int {
	br := wantBitrate(version, layer, brIdx)
	sr := mpegSampleRates[version][srIdx]
	switch layer {
	case 3: // Layer I
		return (12*br*1000/sr + padding) * 4
	case 2: // Layer II
		return 144*br*1000/sr + padding
	default: // Layer III (layer == 1)
		if version == 3 {
			return 144*br*1000/sr + padding
		}
		return 72*br*1000/sr + padding
	}
}

func validFrame() []byte {
	h := mpegHeader(3, 1, 9, 0, 0) // MPEG1 Layer III, 128 kbps, 44,1 kHz
	f := append([]byte{}, h...)
	f = append(f, bytes.Repeat([]byte{0xAA}, mp3FrameLen(3, 1, 9, 0, 0)-4)...)
	return f
}

// TestMPEGFuzzHeaderSpace parcourt TOUTES les combinaisons d'en-têtes.
// Chaque sous-test écrit un fichier [en-tête][corps][5 frames valides] :
//   - combinaison valide → l'en-tête est consommé comme frame (débit attendu,
//     durée > 0) puis les 5 frames valides ;
//   - combinaison invalide → l'en-tête est sauté sans division par zéro ni
//     boucle, et seules les 5 frames valides comptent (débit 128).
func TestMPEGFuzzHeaderSpace(t *testing.T) {
	dir := t.TempDir()
	for version := 0; version < 4; version++ {
		for layer := 0; layer < 4; layer++ {
			for brIdx := 0; brIdx < 16; brIdx++ {
				for srIdx := 0; srIdx < 4; srIdx++ {
					for pad := 0; pad < 2; pad++ {
						name := fmt.Sprintf("v%d_l%d_br%d_sr%d_p%d", version, layer, brIdx, srIdx, pad)
						t.Run(name, func(t *testing.T) {
							audio := append([]byte{}, mpegHeader(version, layer, brIdx, srIdx, pad)...)
							if validCombo(version, layer, brIdx, srIdx) {
								// corps complet de la frame déclarée
								audio = append(audio, bytes.Repeat([]byte{0xAA},
									mp3FrameLen(version, layer, brIdx, srIdx, pad)-4)...)
							} else {
								audio = append(audio, bytes.Repeat([]byte{0xAA}, 300)...)
							}
							for i := 0; i < 5; i++ {
								audio = append(audio, validFrame()...)
							}
							path := filepath.Join(dir, name+".mp3")
							if err := os.WriteFile(path, audio, 0o644); err != nil {
								t.Fatal(err)
							}
							d, br := MPEGDurationBitrate(path) // ne doit ni paniquer ni boucler
							if d < 0 || br < 0 {
								t.Fatalf("valeurs négatives : durée=%d débit=%d", d, br)
							}
							if validCombo(version, layer, brIdx, srIdx) {
								if want := wantBitrate(version, layer, brIdx); br != want {
									t.Errorf("débit = %d, want %d", br, want)
								}
								// 6 frames : 1 (l'en-tête) + 5 valides. Durées
								// possibles : 384×1000/44100 ≈ 8,7 ms à
								// 1152×1000/8000 = 144 ms par frame.
								if d < 6*8 || d > 6*145 {
									t.Errorf("durée = %d ms, want ≈ 6 frames (entre %d et %d)",
										d, 6*8, 6*145)
								}
							} else if br != 128 {
								t.Errorf("débit = %d, want 128 (en-tête invalide sauté, frame valide suivante)", br)
							}
						})
					}
				}
			}
		}
	}
}

// TestMPEGFuzzGarbage : des fichiers aléatoires truffés de faux sync words
// ne doivent ni paniquer ni boucler (le parseur saute, ne s'accroche pas).
func TestMPEGFuzzGarbage(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	dir := t.TempDir()
	for i := 0; i < 300; i++ {
		n := rng.Intn(4096)
		buf := make([]byte, n)
		rng.Read(buf)
		for j := 0; j < n/97; j++ {
			buf[rng.Intn(n)] = 0xFF
		}
		path := filepath.Join(dir, fmt.Sprintf("g%d.mp3", i))
		if err := os.WriteFile(path, buf, 0o644); err != nil {
			t.Fatal(err)
		}
		d, br := MPEGDurationBitrate(path)
		if d < 0 || br < 0 {
			t.Fatalf("fichier %d : valeurs négatives durée=%d débit=%d", i, d, br)
		}
	}
}

// TestMPEGFuzzTruncated : fichiers coupés en plein en-tête ou en plein corps
// de frame — le parseur doit s'arrêter proprement sur EOF.
func TestMPEGFuzzTruncated(t *testing.T) {
	cases := [][]byte{
		{},
		{0xFF},
		{0xFF, 0xFB},
		{0xFF, 0xFB, 0x90},
		{0xFF, 0xFB, 0x90, 0x00},              // en-tête complet, corps absent
		{0xFF, 0xFB, 0x90, 0x00, 0xAA, 0xAA},  // corps déclaré plus long que le fichier
		bytes.Repeat([]byte{0xFF}, 1000),      // que des faux sync
		bytes.Repeat([]byte{0xFF, 0xE0}, 500), // version/layer réservés
		append(validFrame(), validFrame()[:10]...),              // dernière frame coupée
		append(validFrame(), bytes.Repeat([]byte{0x00}, 64)...), // padding
	}
	dir := t.TempDir()
	for i, c := range cases {
		path := filepath.Join(dir, fmt.Sprintf("t%d.mp3", i))
		if err := os.WriteFile(path, c, 0o644); err != nil {
			t.Fatal(err)
		}
		d, br := MPEGDurationBitrate(path) // ne doit ni paniquer ni boucler
		if d < 0 || br < 0 {
			t.Fatalf("cas %d : valeurs négatives durée=%d débit=%d", i, d, br)
		}
	}
}

// TestAverageBitrateKbps : débit moyen (parité mutagen) sur un CBR 128 kbps
// et sur un fichier mélangeant deux débits (CBR 128 puis CBR 320).
func TestAverageBitrateKbps(t *testing.T) {
	dir := t.TempDir()

	// CBR 128 kbps × 40 frames.
	path := filepath.Join(dir, "cbr.mp3")
	var audio []byte
	for i := 0; i < 40; i++ {
		audio = append(audio, validFrame()...)
	}
	if err := os.WriteFile(path, audio, 0o644); err != nil {
		t.Fatal(err)
	}
	if br := AverageBitrateKbps(path); br != 128 {
		t.Errorf("CBR : débit moyen = %d, want 128", br)
	}

	// 20 frames 128 kbps puis 20 frames 320 kbps (MPEG1 LIII, brIdx=14).
	h320 := mpegHeader(3, 1, 14, 0, 0)
	f320 := append([]byte{}, h320...)
	f320 = append(f320, bytes.Repeat([]byte{0xAA}, mp3FrameLen(3, 1, 14, 0, 0)-4)...)
	var mixed []byte
	for i := 0; i < 20; i++ {
		mixed = append(mixed, validFrame()...)
	}
	for i := 0; i < 20; i++ {
		mixed = append(mixed, f320...)
	}
	path = filepath.Join(dir, "vbr.mp3")
	if err := os.WriteFile(path, mixed, 0o644); err != nil {
		t.Fatal(err)
	}
	// moyenne = (20×128 + 20×320) / 40 : les longueurs de frame sont
	// proportionnelles au débit (417 vs 1044 octets) → bits/durée ≈ 223,7 → 224.
	if br := AverageBitrateKbps(path); br != 224 {
		t.Errorf("mixte : débit moyen = %d, want 224", br)
	}
}

// TestAverageBitrateKbpsXing : en-tête Xing/Info (Layer III) — le débit est
// calculé depuis l'en-tête (parité mutagen _parse_vbr_header), PAS par un
// parcours des frames : un fichier « 1 frame Xing 128 kbps + 40 frames
// 320 kbps » donne 320 kbps (319 725 bps → round/1000), et le CBR pur de
// TestAverageBitrateKbps donne 128.
func TestAverageBitrateKbpsXing(t *testing.T) {
	dataLen := 144 * 320000 / 44100 // 1044 octets
	xingLen := 144 * 128000 / 44100 // 417 octets

	first := append([]byte{0xFF, 0xFB, 0x90, 0x00}, bytes.Repeat([]byte{0xAA}, xingLen-4)...)
	audio := append([]byte{}, first...)
	for i := 0; i < 40; i++ { // 40 frames 320 kbps (brIdx=14 → 0xE0)
		audio = append(audio, 0xFF, 0xFB, 0xE0, 0x00)
		audio = append(audio, bytes.Repeat([]byte{0xAA}, dataLen-4)...)
	}
	// En-tête Xing à l'offset 4+32=36 (MPEG1 stéréo) : flags=0x03 (frames+
	// bytes), frames=40, bytes = 417 + 40×1044.
	total := xingLen + 40*dataLen
	hdr := append([]byte{'X', 'i', 'n', 'g', 0, 0, 0, 3},
		byte(40>>24), byte(40>>16), byte(40>>8), byte(40))
	hdr = append(hdr, byte(total>>24), byte(total>>16), byte(total>>8), byte(total))
	copy(audio[36:], hdr)

	path := filepath.Join(t.TempDir(), "xing.mp3")
	if err := os.WriteFile(path, audio, 0o644); err != nil {
		t.Fatal(err)
	}
	// Parcours de frames : (417×8 + 40×1044×8)/(41×26,12 ms) ≈ 315 kbps.
	// En-tête Xing : (42177−417)×8×44100/(1152×40) = 319 725 bps → 320.
	if br := AverageBitrateKbps(path); br != 320 {
		t.Errorf("débit moyen = %d, want 320 (via en-tête Xing, parité mutagen)", br)
	}

	// Même fichier sans l'en-tête Xing : repli sur le parcours de frames.
	copy(audio[36:], bytes.Repeat([]byte{0xAA}, 16))
	if err := os.WriteFile(path, audio, 0o644); err != nil {
		t.Fatal(err)
	}
	if br := AverageBitrateKbps(path); br != 315 {
		t.Errorf("sans Xing : débit moyen = %d, want 315 (repli frames)", br)
	}
}

// TestAverageBitrateKbpsEmpty : aucun frame valide → 0, sans crash.
func TestAverageBitrateKbpsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.mp3")
	if err := os.WriteFile(path, bytes.Repeat([]byte{0xAA}, 512), 0o644); err != nil {
		t.Fatal(err)
	}
	if br := AverageBitrateKbps(path); br != 0 {
		t.Errorf("débit moyen = %d, want 0", br)
	}
}
