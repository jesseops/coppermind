package importer

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jesseops/coppermind/internal/domain"
)

// ExtractAudioMetadata extracts metadata from an audio file or directory.
func ExtractAudioMetadata(filePath string) (*Extracted, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		return extractAudioMetadataFromDir(filePath), nil
	}
	return extractAudioMetadataFromFile(filePath), nil
}

func extractAudioMetadataFromFile(filePath string) *Extracted {
	title := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	title = strings.ReplaceAll(title, "_", " ")
	title = strings.ReplaceAll(title, "-", " ")
	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(filePath)), ".")

	return &Extracted{
		Title:  strings.TrimSpace(title),
		Format: format,
	}
}

func extractAudioMetadataFromDir(dirPath string) *Extracted {
	title := filepath.Base(dirPath)
	title = strings.ReplaceAll(title, "_", " ")
	title = strings.ReplaceAll(title, "-", " ")
	format := inferAudioFormatFromDir(dirPath)

	return &Extracted{
		Title:  strings.TrimSpace(title),
		Format: format,
	}
}

func inferAudioFormatFromDir(dirPath string) string {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return ""
	}
	formats := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(entry.Name())), ".")
		if ext == "mp3" || ext == "m4b" {
			formats[ext] = true
		}
	}
	if len(formats) == 1 {
		for f := range formats {
			return f
		}
	}
	return ""
}

// IsAudioExt returns true if the file extension is a supported audio format.
func IsAudioExt(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3", ".m4b":
		return true
	}
	return false
}

// IsImportableExt returns true if the file extension is importable.
func IsImportableExt(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".epub", ".mobi", ".mp3", ".m4b", ".pdf":
		return true
	}
	return false
}

// ContainsAudioFiles returns true if the directory contains audio files.
func ContainsAudioFiles(dirPath string) bool {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() && IsAudioExt(entry.Name()) {
			return true
		}
	}
	return false
}

// BuildAudiobookTracks enumerates audio files and creates a track list.
func BuildAudiobookTracks(destPath string) ([]domain.Track, int, error) {
	info, err := os.Stat(destPath)
	if err != nil {
		return nil, 0, err
	}

	var files []string
	if info.IsDir() {
		entries, err := os.ReadDir(destPath)
		if err != nil {
			return nil, 0, err
		}
		for _, entry := range entries {
			if !entry.IsDir() && IsAudioExt(entry.Name()) {
				files = append(files, filepath.Join(destPath, entry.Name()))
			}
		}
	} else if IsAudioExt(destPath) {
		files = append(files, destPath)
	}

	if len(files) == 0 {
		return nil, 0, nil
	}
	sort.Strings(files)

	tracks := make([]domain.Track, 0, len(files))
	for idx, fp := range files {
		title := strings.TrimSuffix(filepath.Base(fp), filepath.Ext(fp))
		title = strings.ReplaceAll(title, "_", " ")
		title = strings.ReplaceAll(title, "-", " ")

		tracks = append(tracks, domain.Track{
			TrackIndex: idx + 1,
			Title:      strings.TrimSpace(title),
			FilePath:   fp,
		})
	}
	return tracks, 0, nil // duration unknown without ID3 parsing
}

// InferEditionType determines the edition type from a file path.
func InferEditionType(filePath string) string {
	info, err := os.Stat(filePath)
	if err == nil && info.IsDir() {
		if ContainsAudioFiles(filePath) {
			return domain.EditionTypeAudiobook
		}
		return ""
	}
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".epub", ".mobi", ".pdf":
		return domain.EditionTypeEbook
	case ".mp3", ".m4b":
		return domain.EditionTypeAudiobook
	}
	return ""
}

// InferFormat determines the format string from a file path.
func InferFormat(filePath string) string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(filePath)), ".")
}
