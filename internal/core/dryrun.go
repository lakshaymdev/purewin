package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// DryRunItem represents a single file or directory that would be deleted.
type DryRunItem struct {
	Path     string
	Size     int64
	Category string
}

// DryRunContext tracks what WOULD be deleted during a dry-run.
type DryRunContext struct {
	Items []DryRunItem
	mu    sync.Mutex
}

// NewDryRunContext creates a new empty dry-run context.
func NewDryRunContext() *DryRunContext {
	return &DryRunContext{
		Items: make([]DryRunItem, 0),
	}
}

// Add records a file or directory that would be deleted.
func (d *DryRunContext) Add(path string, size int64, category string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Items = append(d.Items, DryRunItem{
		Path:     path,
		Size:     size,
		Category: category,
	})
}

// TotalSize returns the total bytes that would be freed.
func (d *DryRunContext) TotalSize() int64 {
	d.mu.Lock()
	defer d.mu.Unlock()

	var total int64
	for _, item := range d.Items {
		total += item.Size
	}
	return total
}

// Count returns the total number of items tracked.
func (d *DryRunContext) Count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.Items)
}

// categorySummary groups items by category and calculates totals.
func (d *DryRunContext) categorySummary() map[string]struct {
	count int
	size  int64
} {
	summary := make(map[string]struct {
		count int
		size  int64
	})
	for _, item := range d.Items {
		entry := summary[item.Category]
		entry.count++
		entry.size += item.Size
		summary[item.Category] = entry
	}
	return summary
}

// PrintSummary prints a categorized summary of what would be deleted.
func (d *DryRunContext) PrintSummary() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.Items) == 0 {
		fmt.Println("  Nothing to clean.")
		return
	}

	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════════╗")
	fmt.Println("  ║           DRY RUN — No files deleted     ║")
	fmt.Println("  ╚══════════════════════════════════════════╝")
	fmt.Println()

	summary := d.categorySummary()

	// Sort categories for stable output.
	cats := make([]string, 0, len(summary))
	for cat := range summary {
		cats = append(cats, cat)
	}
	sort.Strings(cats)

	for _, cat := range cats {
		entry := summary[cat]
		fmt.Printf("  %-20s  %5d items  %10s\n",
			strings.ToUpper(cat),
			entry.count,
			FormatSize(entry.size),
		)
	}

	fmt.Println("  ──────────────────────────────────────────")
	fmt.Printf("  %-20s  %5d items  %10s\n",
		"TOTAL",
		len(d.Items),
		FormatSize(d.TotalSizeUnlocked()),
	)
	fmt.Println()
	fmt.Println("  Run without --dry-run to execute cleanup.")
}

// TotalSizeUnlocked calculates total size without acquiring the lock.
// Must only be called while the lock is already held.
func (d *DryRunContext) TotalSizeUnlocked() int64 {
	var total int64
	for _, item := range d.Items {
		total += item.Size
	}
	return total
}

// ExportToFile writes the dry-run results to a text file.
// Default location: %APPDATA%\purewin\clean-list.txt
func (d *DryRunContext) ExportToFile(path string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cannot create export directory %s: %w", dir, err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("PureWin Dry Run Report — %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(strings.Repeat("=", 60) + "\n\n")

	summary := d.categorySummary()
	cats := make([]string, 0, len(summary))
	for cat := range summary {
		cats = append(cats, cat)
	}
	sort.Strings(cats)

	// Group items by category.
	grouped := make(map[string][]DryRunItem)
	for _, item := range d.Items {
		grouped[item.Category] = append(grouped[item.Category], item)
	}

	for _, cat := range cats {
		entry := summary[cat]
		sb.WriteString(fmt.Sprintf("[%s] — %d items, %s\n",
			strings.ToUpper(cat), entry.count, FormatSize(entry.size)))
		for _, item := range grouped[cat] {
			sb.WriteString(fmt.Sprintf("  %10s  %s\n", FormatSize(item.Size), item.Path))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(strings.Repeat("=", 60) + "\n")
	sb.WriteString(fmt.Sprintf("Total: %d items, %s\n",
		len(d.Items), FormatSize(d.TotalSizeUnlocked())))

	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("cannot write export file %s: %w", path, err)
	}

	return nil
}
