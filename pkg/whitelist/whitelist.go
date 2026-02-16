package whitelist

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lakshaymaurya-felt/purewin/internal/envutil"
)

// defaultPatterns are the initial whitelist entries that protect common
// developer tooling from accidental cleanup.
var defaultPatterns = []string{
	`%USERPROFILE%\.cargo\bin\*`,
	`%LOCALAPPDATA%\JetBrains\*`,
	`%APPDATA%\Code\User\*`,
}

// Whitelist manages a set of glob patterns representing paths that
// should be excluded from cleanup operations.
type Whitelist struct {
	patterns []string
	path     string
	mu       sync.RWMutex
}

// Load reads whitelist patterns from the given file path.
// If the file does not exist, a default whitelist is created and saved.
func Load(path string) (*Whitelist, error) {
	w := &Whitelist{
		path:     path,
		patterns: make([]string, 0),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Seed with defaults and persist.
			w.patterns = append(w.patterns, defaultPatterns...)
			if saveErr := w.Save(); saveErr != nil {
				return nil, fmt.Errorf("cannot save default whitelist: %w", saveErr)
			}
			return w, nil
		}
		return nil, fmt.Errorf("cannot read whitelist file %s: %w", path, err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		w.patterns = append(w.patterns, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading whitelist: %w", err)
	}

	return w, nil
}

// Save persists the current whitelist patterns to disk.
func (w *Whitelist) Save() error {
	w.mu.RLock()
	defer w.mu.RUnlock()

	dir := filepath.Dir(w.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cannot create whitelist directory %s: %w", dir, err)
	}

	var sb strings.Builder
	sb.WriteString("# PureWin whitelist — one glob pattern per line\n")
	sb.WriteString("# Lines starting with # are comments\n")
	sb.WriteString("# Environment variables (e.g. %USERPROFILE%) are expanded at runtime\n\n")
	for _, p := range w.patterns {
		sb.WriteString(p + "\n")
	}

	if err := os.WriteFile(w.path, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("cannot write whitelist file %s: %w", w.path, err)
	}
	return nil
}

// validatePattern rejects dangerously broad whitelist patterns that would
// silently prevent all (or most) cleanup operations.
func validatePattern(pattern string) error {
	cleaned := strings.TrimSpace(pattern)

	// 1. Reject wildcard-only patterns.
	if cleaned == "*" || cleaned == "**" {
		return fmt.Errorf("pattern is too broad and would match everything: %s", pattern)
	}

	// 2. Reject drive roots and drive root wildcards (e.g., C:\, C:, C:\*).
	if len(cleaned) >= 2 && cleaned[1] == ':' {
		first := cleaned[0]
		if (first >= 'A' && first <= 'Z') || (first >= 'a' && first <= 'z') {
			rest := ""
			if len(cleaned) > 2 {
				rest = cleaned[2:]
			}
			if rest == "" || rest == `\` || rest == "/" || rest == `\*` || rest == "/*" {
				return fmt.Errorf("pattern is a drive root and too dangerous: %s", pattern)
			}
		}
	}

	// 3. Require at least 2 path separators to avoid overly broad patterns.
	sepCount := strings.Count(cleaned, `\`) + strings.Count(cleaned, "/")
	if sepCount < 2 {
		return fmt.Errorf("pattern has fewer than 2 path separators and is too broad: %s", pattern)
	}

	return nil
}

// Add appends a new pattern to the whitelist.
// Returns an error if the pattern already exists or is dangerously broad.
func (w *Whitelist) Add(pattern string) error {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return fmt.Errorf("pattern cannot be empty")
	}

	// Validate pattern is not dangerously broad.
	if err := validatePattern(pattern); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Check for duplicates (case-insensitive on Windows).
	for _, existing := range w.patterns {
		if strings.EqualFold(existing, pattern) {
			return fmt.Errorf("pattern already exists: %s", pattern)
		}
	}

	w.patterns = append(w.patterns, pattern)
	return nil
}

// Remove deletes a pattern from the whitelist.
// Returns an error if the pattern is not found.
func (w *Whitelist) Remove(pattern string) error {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return fmt.Errorf("pattern cannot be empty")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	for i, existing := range w.patterns {
		if strings.EqualFold(existing, pattern) {
			w.patterns = append(w.patterns[:i], w.patterns[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("pattern not found: %s", pattern)
}

// IsWhitelisted returns true if the given path matches any whitelist
// pattern. Environment variables in patterns are expanded before matching.
func (w *Whitelist) IsWhitelisted(path string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	cleaned := filepath.Clean(path)

	for _, pattern := range w.patterns {
		expanded := envutil.ExpandWindowsEnv(pattern)
		expanded = filepath.Clean(expanded)

		// Exact match (case-insensitive).
		if strings.EqualFold(cleaned, expanded) {
			return true
		}

		// Glob match.
		matched, err := filepath.Match(strings.ToLower(expanded), strings.ToLower(cleaned))
		if err == nil && matched {
			return true
		}

		// Prefix match: if the pattern is a directory (no glob chars),
		// check if path is under it.
		if !strings.ContainsAny(expanded, "*?[") {
			prefix := strings.ToLower(expanded) + string(os.PathSeparator)
			if strings.HasPrefix(strings.ToLower(cleaned)+string(os.PathSeparator), prefix) {
				return true
			}
		}
	}

	return false
}

// List returns a copy of all current whitelist patterns.
func (w *Whitelist) List() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	result := make([]string, len(w.patterns))
	copy(result, w.patterns)
	return result
}
