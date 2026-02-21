package purge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/lakshaymaurya-felt/purewin/internal/core"
)

// ProjectArtifact represents a build artifact found in a project directory.
type ProjectArtifact struct {
	ProjectPath  string    // Path to the project root
	ArtifactPath string    // Full path to the artifact (node_modules, target, etc.)
	ArtifactType string    // Type of artifact (node_modules, target, dist, etc.)
	Size         int64     // Size in bytes
	ModTime      time.Time // Last modification time
	IsRecent     bool      // True if modified within 7 days
}

// artifactDefinition describes how to detect and identify artifacts.
type artifactDefinition struct {
	// DirName is the directory name to look for
	DirName string
	// Type is the user-facing artifact type name
	Type string
	// Indicators are files that should exist at the project root to confirm
	// this is the correct project type. Empty means no check needed.
	Indicators []string
}

// artifactDefinitions lists all artifact types we can detect.
var artifactDefinitions = []artifactDefinition{
	{DirName: "node_modules", Type: "node_modules", Indicators: []string{"package.json"}},
	{DirName: "target", Type: "target", Indicators: []string{"Cargo.toml", "pom.xml"}},
	{DirName: "build", Type: "build", Indicators: []string{"build.gradle", "build.gradle.kts"}},
	{DirName: "dist", Type: "dist", Indicators: []string{"package.json", "vite.config.js", "webpack.config.js"}},
	{DirName: ".next", Type: ".next", Indicators: []string{"next.config.js"}},
	{DirName: ".nuxt", Type: ".nuxt", Indicators: []string{"nuxt.config.js", "nuxt.config.ts"}},
	{DirName: "__pycache__", Type: "__pycache__", Indicators: []string{}},
	{DirName: "venv", Type: "venv", Indicators: []string{}},
	{DirName: ".venv", Type: ".venv", Indicators: []string{}},
	{DirName: ".gradle", Type: ".gradle", Indicators: []string{"build.gradle"}},
	{DirName: ".idea", Type: ".idea", Indicators: []string{}},
	{DirName: "vendor", Type: "vendor", Indicators: []string{"go.mod", "composer.json"}},
	{DirName: "bin", Type: "bin", Indicators: []string{"*.csproj"}},
	{DirName: "obj", Type: "obj", Indicators: []string{"*.csproj"}},
}

// artifactDirNames returns just the directory names for quick checking.
var artifactDirNames = func() map[string]bool {
	m := make(map[string]bool)
	for _, def := range artifactDefinitions {
		m[def.DirName] = true
	}
	return m
}()

// ScanProjects walks the given paths and identifies project artifacts.
// It will scan up to 3 levels deep and NOT recurse into artifact directories.
func ScanProjects(paths []string) ([]ProjectArtifact, error) {
	var artifacts []ProjectArtifact
	seenProjects := make(map[string]bool)

	for _, basePath := range paths {
		basePath = os.ExpandEnv(basePath)
		if _, err := os.Stat(basePath); os.IsNotExist(err) {
			continue // Skip non-existent paths
		}

		err := scanDirectory(basePath, basePath, 0, 3, seenProjects, &artifacts)
		if err != nil {
			// Non-fatal: log but continue scanning other paths
			continue
		}
	}

	// Mark recent artifacts (modified within 7 days)
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	for i := range artifacts {
		if artifacts[i].ModTime.After(cutoff) {
			artifacts[i].IsRecent = true
		}
	}

	return artifacts, nil
}

// isReparsePoint returns true if the path is a Windows junction or symlink.
// Returns true on error (fail-closed) — safer for destructive operations.
func isReparsePoint(path string) bool {
	pathp, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return true // fail-closed: skip on error
	}
	attrs, err := syscall.GetFileAttributes(pathp)
	if err != nil {
		return true // fail-closed: skip on error
	}
	const fileAttributeReparsePoint = 0x0400
	return attrs&fileAttributeReparsePoint != 0
}

// scanDirectory recursively scans a directory for project artifacts.
// depth starts at 0 and increases with each level.
// maxDepth limits how deep we search (typically 3).
func scanDirectory(basePath, currentPath string, depth, maxDepth int, seenProjects map[string]bool, artifacts *[]ProjectArtifact) error {
	if depth > maxDepth {
		return nil
	}

	entries, err := os.ReadDir(currentPath)
	if err != nil {
		// Skip directories we can't read
		return nil
	}

	// Check if current directory contains any artifacts
	projectRoot := currentPath
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()

		// Skip hidden directories (except our specific targets)
		if strings.HasPrefix(name, ".") && name != ".next" && name != ".nuxt" && name != ".venv" && name != ".gradle" && name != ".idea" {
			continue
		}

		// Check if this is an artifact directory
		if !artifactDirNames[name] {
			continue
		}

		artifactPath := filepath.Join(currentPath, name)

		// Find the matching definition
		var def *artifactDefinition
		for i := range artifactDefinitions {
			if artifactDefinitions[i].DirName == name {
				def = &artifactDefinitions[i]
				break
			}
		}
		if def == nil {
			continue
		}

		// Verify project indicators if specified
		if len(def.Indicators) > 0 {
			if !hasAnyIndicator(currentPath, def.Indicators) {
				continue
			}
		}

		// Get size and mod time
		info, err := os.Stat(artifactPath)
		if err != nil {
			continue
		}

		size, err := core.GetDirSize(artifactPath)
		if err != nil {
			// If we can't calculate size, use 0 but still track it
			size = 0
		}

		// Avoid duplicates
		key := strings.ToLower(artifactPath)
		if seenProjects[key] {
			continue
		}
		seenProjects[key] = true

		artifact := ProjectArtifact{
			ProjectPath:  projectRoot,
			ArtifactPath: artifactPath,
			ArtifactType: def.Type,
			Size:         size,
			ModTime:      info.ModTime(),
		}

		*artifacts = append(*artifacts, artifact)
	}

	// Recurse into subdirectories (but not into artifact directories)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()

		// Skip artifact directories - don't recurse into them
		if artifactDirNames[name] {
			continue
		}

		// Skip hidden directories
		if strings.HasPrefix(name, ".") {
			continue
		}

		// Skip common non-project directories
		if name == "node_modules" || name == "target" || name == "dist" {
			continue
		}

		subPath := filepath.Join(currentPath, name)

		// Skip junctions and symlinks — avoid infinite recursion and out-of-scope deletion.
		if isReparsePoint(subPath) {
			continue
		}

		_ = scanDirectory(basePath, subPath, depth+1, maxDepth, seenProjects, artifacts)
	}

	return nil
}

// hasAnyIndicator checks if any of the indicator files/patterns exist in the directory.
func hasAnyIndicator(dir string, indicators []string) bool {
	for _, indicator := range indicators {
		if strings.Contains(indicator, "*") {
			// Glob pattern
			matches, err := filepath.Glob(filepath.Join(dir, indicator))
			if err == nil && len(matches) > 0 {
				return true
			}
		} else {
			// Exact filename
			if _, err := os.Stat(filepath.Join(dir, indicator)); err == nil {
				return true
			}
		}
	}
	return false
}

// PurgeArtifacts deletes the specified artifacts and returns total bytes freed and count.
func PurgeArtifacts(artifacts []ProjectArtifact, dryRun bool) (int64, int, error) {
	var totalBytes int64
	var totalCount int
	var lastErr error

	for _, artifact := range artifacts {
		freed, err := core.SafeDelete(artifact.ArtifactPath, dryRun)
		if err != nil {
			lastErr = err
			continue
		}
		totalBytes += freed
		totalCount++
	}

	return totalBytes, totalCount, lastErr
}

// GetDefaultScanPaths returns the default paths to scan for projects.
func GetDefaultScanPaths() []string {
	userProfile := os.Getenv("USERPROFILE")
	if userProfile == "" {
		userProfile = os.Getenv("HOME")
	}

	return []string{
		filepath.Join(userProfile, "Projects"),
		filepath.Join(userProfile, "GitHub"),
		filepath.Join(userProfile, "dev"),
		filepath.Join(userProfile, "Code"),
		filepath.Join(userProfile, "workspace"),
		filepath.Join(userProfile, "Documents"),
	}
}

// LoadCustomScanPaths reads custom scan paths from the config file.
// Returns empty slice if file doesn't exist.
func LoadCustomScanPaths(configDir string) ([]string, error) {
	pathsFile := filepath.Join(configDir, "purge_paths")
	data, err := os.ReadFile(pathsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read purge_paths: %w", err)
	}

	var paths []string
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Expand environment variables
		line = os.ExpandEnv(line)
		paths = append(paths, line)
	}

	return paths, nil
}

// SaveCustomScanPaths writes custom scan paths to the config file.
func SaveCustomScanPaths(configDir string, paths []string) error {
	pathsFile := filepath.Join(configDir, "purge_paths")

	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	var lines []string
	lines = append(lines, "# Custom paths for project purge scanning")
	lines = append(lines, "# One path per line, environment variables like %USERPROFILE% are supported")
	lines = append(lines, "")
	lines = append(lines, paths...)

	content := strings.Join(lines, "\n")
	if err := os.WriteFile(pathsFile, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write purge_paths: %w", err)
	}

	return nil
}
