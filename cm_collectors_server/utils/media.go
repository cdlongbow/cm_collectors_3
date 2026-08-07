package utils

import (
	"path/filepath"
	"strings"
)

var clearlyNonVideoExtensions = map[string]struct{}{
	".jpg": {}, ".jpeg": {}, ".png": {}, ".gif": {}, ".bmp": {}, ".webp": {}, ".svg": {}, ".avif": {}, ".heic": {},
	".html": {}, ".htm": {}, ".nfo": {}, ".txt": {}, ".json": {}, ".xml": {}, ".url": {},
	".srt": {}, ".ass": {}, ".ssa": {}, ".vtt": {}, ".sub": {},
}

func ClearlyNonVideoExtensions() []string {
	result := make([]string, 0, len(clearlyNonVideoExtensions))
	for extension := range clearlyNonVideoExtensions {
		result = append(result, extension)
	}
	return result
}

// IsClearlyNonVideoSource only rejects formats that are unambiguously not video.
// Unknown extensions remain probeable so uncommon video containers and stream URLs keep working.
func IsClearlyNonVideoSource(src string) bool {
	cleaned := src
	if index := strings.IndexAny(cleaned, "?#"); index >= 0 {
		cleaned = cleaned[:index]
	}
	extension := strings.ToLower(filepath.Ext(cleaned))
	_, exists := clearlyNonVideoExtensions[extension]
	return exists
}
