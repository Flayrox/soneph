// ── MPEG frame parsing (Layer I/II/III, MPEG1/2/2.5) ──────────────────────
// Suffisant pour une durée et un débit honnêtes : on parcourt les frames en
// s'appuyant sur les tables bitrate/échantillonnage du standard ISO 11172-3.
// Déplacé depuis pkg/store/tags.go (M6) : partagé entre l'enrichissement du
// store et le dump FileDetails du panneau « Plus de détails ».

package tags

import (
	"bufio"
	"io"
	"math"
	"os"
)

var mpegBitrateL3 = [16]int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0}
var mpegBitrateL3Low = [16]int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0}
var mpegSampleRates = [4][4]int{
	{11025, 12000, 8000, 0}, // MPEG 2.5
	{0, 0, 0, 0},
	{22050, 24000, 16000, 0}, // MPEG 2
	{44100, 48000, 32000, 0}, // MPEG 1
}

// walkMPEGFrames ouvre un MP3 et appelle fn pour chaque en-tête de frame
// valide rencontré (Layer I/II/III, MPEG1/2/2.5). C'est l'unique parcours :
// MPEGDurationBitrate (durée + premier débit) et AverageBitrateKbps (débit
// moyen, parité mutagen) l'utilisent. Toute erreur de lecture met fin au
// parcours sans erreur — un fichier malformé ne fait jamais paniquer ni
// boucler le scan.
func walkMPEGFrames(path string, fn func(frameLen, samplesPerFrame, sr, bitrate int)) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	r := bufio.NewReaderSize(f, 64*1024)
	for {
		// Cherche le sync word 0xFF Ex (11 bits à 1).
		b, err := r.ReadByte()
		if err != nil {
			return
		}
		if b != 0xFF {
			continue
		}
		b2, err := r.ReadByte()
		if err != nil {
			return
		}
		if b2&0xE0 != 0xE0 {
			continue
		}
		header := []byte{b, b2}
		rest := make([]byte, 2)
		if _, err := io.ReadFull(r, rest); err != nil {
			return
		}
		header = append(header, rest...)

		version := (header[1] >> 3) & 0x03 // 3=MPEG1, 2=MPEG2, 0=MPEG2.5
		if version == 1 {
			continue // réservé
		}
		layer := (header[1] >> 1) & 0x03
		if layer == 0 {
			continue // réservé
		}
		brIdx := (header[2] >> 4) & 0x0F
		srIdx := (header[2] >> 2) & 0x03
		padding := int((header[2] >> 1) & 0x01)

		// En-tête invalide (ex. srIdx=3 → sr=0, ou brIdx=0/15 → br=0) : on
		// saute AVANT toute division — un fichier réel avec un en-tête
		// inhabituel ne doit jamais faire paniquer le scan de démarrage.
		var br, sr, frameLen int
		var samplesPerFrame int
		if layer == 3 { // Layer I
			br = mpegBitrateL3[brIdx]
			sr = mpegSampleRates[version][srIdx]
			if br == 0 || sr == 0 {
				continue
			}
			frameLen = (12*br*1000/sr + padding) * 4
			samplesPerFrame = 384
		} else if layer == 2 { // Layer II
			if version == 3 {
				br = mpegBitrateL3[brIdx]
			} else {
				br = mpegBitrateL3Low[brIdx]
			}
			sr = mpegSampleRates[version][srIdx]
			if br == 0 || sr == 0 {
				continue
			}
			frameLen = 144 * br * 1000 / sr
			if padding == 1 {
				frameLen++
			}
			samplesPerFrame = 1152
		} else { // Layer III
			br = mpegBitrateL3[brIdx]
			if version != 3 {
				br = mpegBitrateL3Low[brIdx]
			}
			sr = mpegSampleRates[version][srIdx]
			if br == 0 || sr == 0 {
				continue
			}
			if version == 3 {
				frameLen = 144*br*1000/sr + padding
				samplesPerFrame = 1152
			} else {
				frameLen = 72*br*1000/sr + padding
				samplesPerFrame = 576
			}
		}
		if frameLen <= 4 {
			continue
		}
		fn(frameLen, samplesPerFrame, sr, br)
		if _, err := r.Discard(frameLen - 4); err != nil {
			return
		}
	}
}

// MPEGDurationBitrate ouvre un MP3 et calcule durée (ms) et débit (kbps)
// depuis les frames audio. Retourne (0,0) si aucun frame valide.
func MPEGDurationBitrate(path string) (durationMs int, bitrate int) {
	bitrate = -1
	walkMPEGFrames(path, func(frameLen, samplesPerFrame, sr, br int) {
		if bitrate < 0 {
			bitrate = br
		}
		durationMs += samplesPerFrame * 1000 / sr
	})
	if bitrate < 0 {
		return 0, 0
	}
	return durationMs, bitrate
}

// AverageBitrateKbps calcule le débit moyen du fichier — parité mutagen
// MP3.info.bitrate :
//   - Layer III avec en-tête VBR (Xing/Info ou VBRI) : moyenne calculée depuis
//     l'en-tête (octets × 8 × échantillonnage / (spf × frames)), PAS un
//     parcours des frames — mutagen fait exactement cela, et c'est ce qui
//     rend l'étiquette TXXX:SONEPH_QUALITY identique à celle que tag_soneph.py
//     écrivait (sinon 319 vs 320 kbps sur les fichiers spotdl VBR).
//   - sinon : bits audio totaux / durée totale (CBR → débit de frame).
//
// 0 si aucune frame valide. C'est la valeur inscrite dans le tag
// TXXX:SONEPH_QUALITY par StampSoneph (port de tag_soneph.py).
func AverageBitrateKbps(path string) int {
	if br := averageFromVBRHeader(path); br > 0 {
		return br
	}
	var totalBits, totalMs float64
	walkMPEGFrames(path, func(frameLen, samplesPerFrame, sr, br int) {
		totalBits += float64(frameLen) * 8
		totalMs += float64(samplesPerFrame) * 1000 / float64(sr)
	})
	if totalMs == 0 {
		return 0
	}
	return int(math.Round(totalBits / totalMs)) // bits/ms == kbps
}

// averageFromVBRHeader lit le débit moyen depuis l'en-tête VBR du premier
// frame (Xing/Info ou VBRI, Layer III uniquement) — la formule exacte de
// mutagen (_parse_vbr_header). 0 si pas d'en-tête VBR exploitable.
func averageFromVBRHeader(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	// Saute le tag ID3 (10 octets + taille synchsafe) ; frameStart = début
	// du premier frame audio.
	frameStart := int64(0)
	hdr := make([]byte, 10)
	if _, err := io.ReadFull(f, hdr); err != nil {
		return 0
	}
	if string(hdr[0:3]) == "ID3" {
		size := int(hdr[6])<<21 | int(hdr[7])<<14 | int(hdr[8])<<7 | int(hdr[9])
		frameStart = 10 + int64(size)
	}

	fh := make([]byte, 4)
	if _, err := f.ReadAt(fh, frameStart); err != nil {
		return 0
	}
	if fh[0] != 0xFF || fh[1]&0xE0 != 0xE0 {
		return 0
	}
	version := (fh[1] >> 3) & 0x03 // 3=MPEG1, 2=MPEG2, 0=MPEG2.5
	layer := (fh[1] >> 1) & 0x03
	if version == 1 || layer != 1 { // Xing/VBRI n'existent qu'en Layer III
		return 0
	}
	srIdx := (fh[2] >> 2) & 0x03
	sr := mpegSampleRates[version][srIdx]
	if sr == 0 {
		return 0
	}
	brIdx := (fh[2] >> 4) & 0x0F
	firstBR := mpegBitrateL3[brIdx]
	if version != 3 {
		firstBR = mpegBitrateL3Low[brIdx]
	}
	if firstBR == 0 {
		return 0
	}
	padding := int((fh[2] >> 1) & 0x01)
	firstLen := 144*firstBR*1000/sr + padding
	spf := 1152
	if version != 3 {
		firstLen = 72*firstBR*1000/sr + padding
		spf = 576
	}

	// Offset du champ VBR dans le premier frame : 4 (en-tête) + side info
	// (mono = 17/9 octets, stéréo = 32/17 — même table que mutagen).
	mode := (fh[3] >> 6) & 0x03
	xingOff := int64(4 + 17)
	if version == 3 {
		if mode == 3 {
			xingOff = 4 + 17
		} else {
			xingOff = 4 + 32
		}
	} else if mode != 3 {
		xingOff = 4 + 17
	}

	magic := make([]byte, 4)
	if _, err := f.ReadAt(magic, frameStart+xingOff); err != nil {
		return 0
	}
	switch string(magic) {
	case "Xing", "Info":
		flagsB := make([]byte, 4)
		if _, err := f.ReadAt(flagsB, frameStart+xingOff+4); err != nil {
			return 0
		}
		flags := int(flagsB[0])<<24 | int(flagsB[1])<<16 | int(flagsB[2])<<8 | int(flagsB[3])
		var frames, xbytes int
		off := frameStart + xingOff + 8
		if flags&0x01 != 0 { // champ « frames »
			b := make([]byte, 4)
			if _, err := f.ReadAt(b, off); err != nil {
				return 0
			}
			frames = int(b[0])<<24 | int(b[1])<<16 | int(b[2])<<8 | int(b[3])
			off += 4
		}
		if flags&0x02 != 0 { // champ « bytes »
			b := make([]byte, 4)
			if _, err := f.ReadAt(b, off); err != nil {
				return 0
			}
			xbytes = int(b[0])<<24 | int(b[1])<<16 | int(b[2])<<8 | int(b[3])
		}
		if frames <= 0 {
			return 0
		}
		// mutagen : audio_bytes = bytes − première frame (comptée dans
		// bytes mais pas dans frames) ; bitrate = audio_bytes×8×sr/(spf×frames)
		// en bps → /1000 pour du kbps (tag_soneph.py faisait round(bps/1000)).
		audioBytes := xbytes - firstLen
		if audioBytes < 0 {
			audioBytes = 0
		}
		return int(math.Round(float64(audioBytes) * 8 * float64(sr) / float64(spf*frames) / 1000))
	case "VBRI":
		b := make([]byte, 8)
		if _, err := f.ReadAt(b, frameStart+xingOff+10); err != nil {
			return 0
		}
		xbytes := int(b[0])<<24 | int(b[1])<<16 | int(b[2])<<8 | int(b[3])
		frames := int(b[4])<<24 | int(b[5])<<16 | int(b[6])<<8 | int(b[7])
		if frames <= 0 {
			return 0
		}
		return int(math.Round(float64(xbytes) * 8 * float64(sr) / float64(spf*frames) / 1000))
	}
	return 0
}
