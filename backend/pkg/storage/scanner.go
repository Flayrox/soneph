package storage

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

type DownloadedFile struct {
	ID        string    `json:"id"`
	FileName  string    `json:"file_name"`
	Title     string    `json:"title"`
	Artist    string    `json:"artist"`
	Album     string    `json:"album"`
	Path      string    `json:"path"`
	RelPath   string    `json:"rel_path"`
	Size      int64     `json:"size"`
	HasLyrics bool      `json:"has_lyrics"`
	LrcPath   string    `json:"lrc_path,omitempty"`
	ModTime   time.Time `json:"mod_time"`
}

type Scanner struct {
	DownloadDir string
}

func NewScanner(downloadDir string) *Scanner {
	if downloadDir == "" {
		downloadDir = "./downloads"
	}
	return &Scanner{DownloadDir: downloadDir}
}

func (s *Scanner) ListFiles() ([]DownloadedFile, error) {
	var files []DownloadedFile

	lrcMap := make(map[string]string)

	// First pass: locate all .lrc files
	_ = filepath.Walk(s.DownloadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(path)) == ".lrc" {
			baseWithoutExt := strings.TrimSuffix(path, filepath.Ext(path))
			lrcMap[baseWithoutExt] = path
		}
		return nil
	})

	// Second pass: locate all audio files (.mp3, .m4a, .flac, .ogg, .wav)
	err := filepath.Walk(s.DownloadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".mp3" || ext == ".m4a" || ext == ".flac" || ext == ".ogg" || ext == ".wav" {
			rel, _ := filepath.Rel(s.DownloadDir, path)
			parts := strings.Split(rel, string(os.PathSeparator))

			artist := "Unknown Artist"
			album := "Unknown Album"
			title := strings.TrimSuffix(info.Name(), filepath.Ext(info.Name()))
			title = strings.TrimSuffix(title, ".{ext}")

			// If saved under /downloads/{artist}/{album}/{title}.mp3
			if len(parts) >= 3 {
				artist = parts[0]
				album = parts[1]
			} else if len(parts) == 2 {
				artist = parts[0]
			}

			baseWithoutExt := strings.TrimSuffix(path, filepath.Ext(path))
			lrcPath, hasLyrics := lrcMap[baseWithoutExt]

			files = append(files, DownloadedFile{
				ID:        rel,
				FileName:  info.Name(),
				Title:     title,
				Artist:    artist,
				Album:     album,
				Path:      path,
				RelPath:   rel,
				Size:      info.Size(),
				HasLyrics: hasLyrics,
				LrcPath:   lrcPath,
				ModTime:   info.ModTime(),
			})
		}
		return nil
	})

	return files, err
}

func (s *Scanner) DeleteFile(relPath string) error {
	fullPath := filepath.Join(s.DownloadDir, filepath.Clean(relPath))
	err := os.Remove(fullPath)
	if err != nil {
		return err
	}

	// Also remove matching .lrc file if exists
	baseWithoutExt := strings.TrimSuffix(fullPath, filepath.Ext(fullPath))
	lrcPath := baseWithoutExt + ".lrc"
	if _, err := os.Stat(lrcPath); err == nil {
		_ = os.Remove(lrcPath)
	}

	return nil
}
