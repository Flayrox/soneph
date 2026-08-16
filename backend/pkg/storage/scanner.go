package storage

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var reLrcTimestamp = regexp.MustCompile(`\[\d{2}:\d{2}\.\d{2,3}\]`)

type DownloadedFile struct {
	ID         string    `json:"id"`
	FileName   string    `json:"file_name"`
	Title      string    `json:"title"`
	Artist     string    `json:"artist"`
	Album      string    `json:"album"`
	Path       string    `json:"path"`
	RelPath    string    `json:"rel_path"`
	Size       int64     `json:"size"`
	HasLyrics  bool      `json:"has_lyrics"`
	LyricsType string    `json:"lyrics_type"` // "synced" | "unsynced" | "none"
	LrcPath    string    `json:"lrc_path,omitempty"`
	ModTime    time.Time `json:"mod_time"`
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

func inspectLyricsType(lrcPath string) (bool, string) {
	if lrcPath == "" {
		return false, "none"
	}
	content, err := os.ReadFile(lrcPath)
	if err != nil || len(content) == 0 {
		return false, "none"
	}
	if reLrcTimestamp.Match(content) {
		return true, "synced"
	}
	return true, "unsynced"
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
			lrcPath := lrcMap[baseWithoutExt]
			hasLyrics, lyricsType := inspectLyricsType(lrcPath)

			files = append(files, DownloadedFile{
				ID:         rel,
				FileName:   info.Name(),
				Title:      title,
				Artist:     artist,
				Album:      album,
				Path:       path,
				RelPath:    rel,
				Size:       info.Size(),
				HasLyrics:  hasLyrics,
				LyricsType: lyricsType,
				LrcPath:    lrcPath,
				ModTime:    info.ModTime(),
			})
		}
		return nil
	})

	return files, err
}

// ResolvePath converts a rel_path from the API into a safe absolute path,
// refusing anything that escapes the downloads directory (../ traversal).
func (s *Scanner) ResolvePath(relPath string) (string, error) {
	clean := filepath.Clean(relPath)
	if clean == "." || clean == ".." || filepath.IsAbs(clean) ||
		strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", ErrInvalidPath
	}
	full := filepath.Join(s.DownloadDir, clean)
	if full != s.DownloadDir && !strings.HasPrefix(full, s.DownloadDir+string(os.PathSeparator)) {
		return "", ErrInvalidPath
	}
	return full, nil
}

// ErrInvalidPath is returned when a rel_path escapes the downloads directory.
var ErrInvalidPath = errors.New("invalid path: must stay inside the downloads directory")

func (s *Scanner) DeleteFile(relPath string) error {
	fullPath, err := s.ResolvePath(relPath)
	if err != nil {
		return err
	}
	err = os.Remove(fullPath)
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
