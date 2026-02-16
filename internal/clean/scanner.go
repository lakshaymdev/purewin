package clean

import (
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/lakshaymaurya-felt/purewin/internal/config"
	"github.com/lakshaymaurya-felt/purewin/pkg/whitelist"
)

// ─── Data Structures ─────────────────────────────────────────────────────────

// CleanItem represents a single file or directory eligible for cleanup.
type CleanItem struct {
	// Path is the absolute filesystem path.
	Path string

	// Size is the size in bytes.
	Size int64

	// Category is the high-level grouping (user, browser, dev, system).
	Category string

	// Description is a human-readable label for the parent target.
	Description string
}

// ScanResult holds the aggregated scan output for a single clean target.
type ScanResult struct {
	// Category is the target name (e.g. "ChromeCache", "NpmCache").
	Category string

	// Items is the list of discovered cleanable files/directories.
	Items []CleanItem

	// TotalSize is the sum of all item sizes in bytes.
	TotalSize int64

	// ItemCount is the number of items discovered.
	ItemCount int
}

// ─── Parallel Scan Engine ────────────────────────────────────────────────────

// ScanAll scans all provided targets in parallel, returning results for each
// target that has cleanable items. Targets requiring admin privileges are
// skipped when isAdmin is false. Whitelisted paths are excluded.
func ScanAll(targets []config.CleanTarget, wl *whitelist.Whitelist, isAdmin bool) []ScanResult {
	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		results []ScanResult
	)

	for _, t := range targets {
		// Skip admin-required targets if not elevated.
		if t.RequiresAdmin && !isAdmin {
			continue
		}

		// RecycleBin has no filesystem paths; handled via Shell API separately.
		if t.Name == "RecycleBin" {
			continue
		}

		wg.Add(1)
		go func(target config.CleanTarget) {
			defer wg.Done()

			items := scanTarget(target, wl)
			if len(items) == 0 {
				return
			}

			result := ItemsToResult(target.Name, items)

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(t)
	}

	wg.Wait()

	// Sort results by category name for stable output.
	sort.Slice(results, func(i, j int) bool {
		return results[i].Category < results[j].Category
	})

	return results
}

// ─── Single-Target Scanning ──────────────────────────────────────────────────

// scanTarget scans a single CleanTarget by resolving environment variables
// and glob patterns in its paths.
func scanTarget(target config.CleanTarget, wl *whitelist.Whitelist) []CleanItem {
	var items []CleanItem

	for _, rawPath := range target.Paths {
		// Expand environment variables.
		expanded := os.ExpandEnv(rawPath)

		// Attempt glob expansion for wildcard patterns.
		matches, err := filepath.Glob(expanded)
		if err != nil || len(matches) == 0 {
			// If glob fails or returns nothing, try the literal path.
			matches = []string{expanded}
		}

		for _, path := range matches {
			path = filepath.Clean(path)

			// Skip whitelisted paths.
			if wl != nil && wl.IsWhitelisted(path) {
				continue
			}

			info, statErr := os.Lstat(path)
			if statErr != nil {
				continue // Path doesn't exist or is inaccessible.
			}

			if info.IsDir() {
				dirItems := scanDirectory(path, target.Category, target.Description, wl)
				items = append(items, dirItems...)
			} else {
				items = append(items, CleanItem{
					Path:        path,
					Size:        info.Size(),
					Category:    target.Category,
					Description: target.Description,
				})
			}
		}
	}

	return items
}

// scanDirectory walks a directory tree collecting all files as CleanItems.
// Whitelisted and inaccessible entries are silently skipped.
func scanDirectory(dir, category, description string, wl *whitelist.Whitelist) []CleanItem {
	var items []CleanItem

	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip inaccessible entries.
		}
		if d.IsDir() {
			return nil
		}

		if wl != nil && wl.IsWhitelisted(path) {
			return nil
		}

		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}

		items = append(items, CleanItem{
			Path:        path,
			Size:        info.Size(),
			Category:    category,
			Description: description,
		})
		return nil
	})

	return items
}

// ─── Aggregation Helpers ─────────────────────────────────────────────────────

// ItemsToResult converts a slice of CleanItems into a ScanResult with
// the given name and pre-calculated totals.
func ItemsToResult(name string, items []CleanItem) ScanResult {
	var totalSize int64
	for _, item := range items {
		totalSize += item.Size
	}
	return ScanResult{
		Category:  name,
		Items:     items,
		TotalSize: totalSize,
		ItemCount: len(items),
	}
}

// GroupByCategory aggregates scan results by the high-level category of
// their items (user, browser, dev, system).
func GroupByCategory(results []ScanResult) map[string][]ScanResult {
	groups := make(map[string][]ScanResult)
	for _, r := range results {
		if len(r.Items) > 0 {
			cat := r.Items[0].Category
			groups[cat] = append(groups[cat], r)
		}
	}
	return groups
}

// TotalSizeAll returns the combined size across all scan results.
func TotalSizeAll(results []ScanResult) int64 {
	var total int64
	for _, r := range results {
		total += r.TotalSize
	}
	return total
}

// TotalItemCount returns the combined item count across all scan results.
func TotalItemCount(results []ScanResult) int {
	var total int
	for _, r := range results {
		total += r.ItemCount
	}
	return total
}
