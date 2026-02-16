package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/lakshaymaurya-felt/purewin/internal/config"
	"github.com/lakshaymaurya-felt/purewin/internal/envutil"
)

// IsSafePath returns true if the given path is NOT in the NEVER_DELETE list.
// Paths are compared case-insensitively after cleaning.
func IsSafePath(path string) bool {
	cleaned := filepath.Clean(path)
	for _, protected := range config.GetNeverDeletePaths() {
		if strings.EqualFold(cleaned, filepath.Clean(protected)) {
			return false
		}
		// Also block anything directly under a never-delete path.
		// e.g. C:\Windows\System32\drivers is still under System32.
		protectedClean := filepath.Clean(protected) + string(os.PathSeparator)
		if strings.HasPrefix(strings.ToLower(cleaned)+string(os.PathSeparator), strings.ToLower(protectedClean)) {
			return false
		}
	}
	return true
}

// ValidatePath performs comprehensive validation on a path before any
// file operation. It returns nil if the path is safe to operate on.
func ValidatePath(path string) error {
	// 1. Not empty.
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path is empty")
	}

	// 2. Must be absolute.
	if !filepath.IsAbs(path) {
		return fmt.Errorf("path must be absolute, got: %s", path)
	}

	// 2.5. Reject drive roots (e.g., C:\ or C:).
	cleaned := filepath.Clean(path)
	if len(cleaned) >= 2 && len(cleaned) <= 3 && cleaned[1] == ':' && unicode.IsLetter(rune(cleaned[0])) {
		return fmt.Errorf("path is a drive root and cannot be operated on: %s", path)
	}

	// 3. No path traversal components.
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == ".." {
			return fmt.Errorf("path contains traversal component (..): %s", path)
		}
	}

	// 4. No control characters.
	for _, r := range path {
		if unicode.IsControl(r) && r != '\t' {
			return fmt.Errorf("path contains control character (U+%04X): %s", r, path)
		}
	}

	// 5. Not a NEVER_DELETE path.
	if !IsSafePath(path) {
		return fmt.Errorf("path is protected and must NEVER be deleted: %s", path)
	}

	// 6. If it exists and is a symlink/junction, resolve and re-check.
	info, err := os.Lstat(path)
	if err == nil && (info.Mode()&os.ModeSymlink != 0) {
		resolved, resolveErr := filepath.EvalSymlinks(path)
		if resolveErr != nil {
			return fmt.Errorf("cannot resolve symlink %s: %w", path, resolveErr)
		}
		if !IsSafePath(resolved) {
			return fmt.Errorf("symlink %s resolves to protected path: %s", path, resolved)
		}
	}

	return nil
}

// IsPathProtected returns true if the path matches any pattern in the
// given whitelist. Patterns support filepath.Match glob syntax.
func IsPathProtected(path string, whitelist []string) bool {
	cleaned := filepath.Clean(path)
	for _, pattern := range whitelist {
		expandedPattern := envutil.ExpandWindowsEnv(pattern)
		expandedPattern = filepath.Clean(expandedPattern)

		// Try exact match (case-insensitive).
		if strings.EqualFold(cleaned, expandedPattern) {
			return true
		}

		// Try glob match.
		matched, err := filepath.Match(strings.ToLower(expandedPattern), strings.ToLower(cleaned))
		if err == nil && matched {
			return true
		}

		// Check if path is under a whitelisted directory.
		prefix := strings.ToLower(expandedPattern)
		if !strings.HasSuffix(prefix, string(os.PathSeparator)) && !strings.ContainsRune(prefix, '*') {
			prefix += string(os.PathSeparator)
		}
		if !strings.ContainsRune(prefix, '*') &&
			strings.HasPrefix(strings.ToLower(cleaned)+string(os.PathSeparator), prefix) {
			return true
		}
	}
	return false
}
