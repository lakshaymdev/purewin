package analyze

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	cacheFileName = "analyze_cache.json"
	cacheTTL      = 5 * time.Minute
)

// cacheEntry wraps a scan result with metadata for validation.
type cacheEntry struct {
	Timestamp time.Time `json:"timestamp"`
	RootPath  string    `json:"root_path"`
	Root      *DirEntry `json:"root"`
	RootMtime time.Time `json:"root_mtime"`
}

// cacheDir returns the %APPDATA%\purewin directory, creating it if needed.
func cacheDir() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	dir := filepath.Join(appData, "purewin")
	return dir, os.MkdirAll(dir, 0o755)
}

// cachePath generates a cache file path keyed by the scan root.
func cachePath(rootPath string) string {
	dir, err := cacheDir()
	if err != nil {
		return ""
	}
	// Sanitize path into a safe filename component.
	safe := strings.NewReplacer(`\`, "_", `/`, "_", `:`, "").Replace(rootPath)
	if len(safe) > 80 {
		safe = safe[:80]
	}
	return filepath.Join(dir, safe+"_"+cacheFileName)
}

// SaveCache persists scan results to disk. Non-sensitive: only paths, sizes,
// and timestamps are stored.
func SaveCache(root *DirEntry, rootPath string) error {
	path := cachePath(rootPath)
	if path == "" {
		return nil
	}

	// Get root directory mtime for invalidation.
	var rootMtime time.Time
	if info, err := os.Stat(rootPath); err == nil {
		rootMtime = info.ModTime()
	}

	entry := cacheEntry{
		Timestamp: time.Now(),
		RootPath:  rootPath,
		Root:      root,
		RootMtime: rootMtime,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// LoadCache loads cached scan results if they exist and haven't expired.
// Returns os.ErrNotExist if no valid cache is found.
func LoadCache(rootPath string) (*DirEntry, error) {
	path := cachePath(rootPath)
	if path == "" {
		return nil, os.ErrNotExist
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}

	// Validate: root path must match.
	if entry.RootPath != rootPath {
		return nil, os.ErrNotExist
	}

	// Validate: cache must not be expired.
	if time.Since(entry.Timestamp) > cacheTTL {
		return nil, os.ErrNotExist
	}

	// Validate: root directory mtime must not have changed.
	// NOTE: Only detects direct child changes (add/remove/rename).
	// Deep tree modifications within the TTL window won't invalidate.
	info, err := os.Stat(rootPath)
	if err != nil || !info.ModTime().Equal(entry.RootMtime) {
		return nil, os.ErrNotExist
	}

	// Rebuild parent pointers (not serialized to avoid circular refs).
	rebuildParents(entry.Root, nil)

	return entry.Root, nil
}

// rebuildParents restores Parent pointers after deserialization.
func rebuildParents(entry *DirEntry, parent *DirEntry) {
	if entry == nil {
		return
	}
	entry.Parent = parent
	for _, child := range entry.Children {
		rebuildParents(child, entry)
	}
}
