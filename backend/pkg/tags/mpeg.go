// ── MPEG frame parsing (Layer I/II/III, MPEG1/2/2.5) ──────────────────────
// Suffisant pour une durée et un débit honnêtes : on parcourt les frames en
// s'appuyant sur les tables bitrate/échantillonnage du standard ISO 11172-3.
// Déplacé depuis pkg/store/tags.go (M6) : partagé entre l'enrichissement du
// store et le dump FileDetails du panneau « Plus de détails ».

package tags

import (
	"bufio"
	"io"
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

// MPEGDurationBitrate ouvre un MP3 et calcule durée (ms) et débit (kbps)
// depuis les frames audio. Retourne (0,0) si aucun frame valide.
func MPEGDurationBitrate(path string) (durationMs int, bitrate int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	r := bufio.NewReaderSize(f, 64*1024)
	bitrate = -1
	for {
		// Cherche le sync word 0xFF Ex (11 bits à 1).
		b, err := r.ReadByte()
		if err != nil {
			break
		}
		if b != 0xFF {
			continue
		}
		b2, err := r.ReadByte()
		if err != nil {
			break
		}
		if b2&0xE0 != 0xE0 {
			continue
		}
		header := []byte{b, b2}
		rest := make([]byte, 2)
		if _, err := io.ReadFull(r, rest); err != nil {
			break
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

		var br, sr, frameLen int
		var samplesPerFrame int
		if layer == 3 { // Layer I
			br = mpegBitrateL3[brIdx]
			sr = mpegSampleRates[version][srIdx]
			frameLen = (12*br*1000/sr + padding) * 4
			samplesPerFrame = 384
		} else if layer == 2 { // Layer II
			if version == 3 {
				br = mpegBitrateL3[brIdx]
			} else {
				br = mpegBitrateL3Low[brIdx]
			}
			sr = mpegSampleRates[version][srIdx]
			frameLen = 144 * br * 1000 / sr
			if padding == 1 {
				frameLen++
			}
			samplesPerFrame = 1152
		} else { // Layer III
			sr = mpegSampleRates[version][srIdx]
			if version == 3 {
				br = mpegBitrateL3[brIdx]
				frameLen = 144*br*1000/sr + padding
				samplesPerFrame = 1152
			} else {
				br = mpegBitrateL3Low[brIdx]
				frameLen = 72*br*1000/sr + padding
				samplesPerFrame = 576
			}
		}
		if br == 0 || sr == 0 || frameLen <= 4 {
			continue
		}
		if bitrate < 0 {
			bitrate = br
		}
		durationMs += samplesPerFrame * 1000 / sr
		if _, err := r.Discard(frameLen - 4); err != nil {
			break
		}
	}
	if bitrate < 0 {
		return 0, 0
	}
	return durationMs, bitrate
}
