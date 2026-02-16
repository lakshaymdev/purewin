package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// unprotectedTempDir creates a temporary directory that passes IsSafePath.
// t.TempDir() creates under C:\Users which is in NEVER_DELETE, so SafeDelete
// would reject those paths. We try drive-root locations instead.
func unprotectedTempDir(t *testing.T) string {
	t.Helper()
	candidates := []string{`C:\PureWinTest`, `D:\PureWinTest`, `E:\PureWinTest`}
	for _, base := range candidates {
		if err := os.MkdirAll(base, 0o755); err != nil {
			continue
		}
		dir, err := os.MkdirTemp(base, "wmt-")
		if err != nil {
			continue
		}
		if !IsSafePath(dir) {
			os.RemoveAll(dir)
			continue
		}
		t.Cleanup(func() {
			os.RemoveAll(dir)
			os.Remove(base) // remove parent if empty
		})
		return dir
	}
	t.Skip("no writable non-protected directory available; skipping file-operation test")
	return ""
}

// ---------------------------------------------------------------------------
// SafeDelete tests
// ---------------------------------------------------------------------------

func TestSafeDelete_RejectsProtectedPaths(t *testing.T) {
	for _, p := range []string{
		`C:\Windows`,
		`C:\Windows\System32`,
		`C:\Users`,
		`C:\Program Files`,
		`C:\ProgramData`,
	} {
		_, err := SafeDelete(p, false)
		if err == nil {
			t.Errorf("SafeDelete(%q) must reject protected path", p)
		}
		if !strings.Contains(err.Error(), "safety check failed") {
			t.Errorf("SafeDelete(%q) error should mention safety check, got: %v", p, err)
		}
	}
}

func TestSafeDelete_DryRunDoesNotDelete(t *testing.T) {
	dir := unprotectedTempDir(t)
	fpath := filepath.Join(dir, "testfile.tmp")
	if err := os.WriteFile(fpath, []byte("dry run test data"), 0o644); err != nil {
		t.Fatalf("cannot create test file: %v", err)
	}

	size, err := SafeDelete(fpath, true)
	if err != nil {
		t.Fatalf("SafeDelete(dryRun=true) returned error: %v", err)
	}
	if size == 0 {
		t.Error("SafeDelete(dryRun=true) should report non-zero size")
	}
	// File must still exist after dry run.
	if _, statErr := os.Stat(fpath); os.IsNotExist(statErr) {
		t.Fatal("file was deleted during dry run — SAFETY VIOLATION")
	}
}

func TestSafeDelete_DeletesValidFile(t *testing.T) {
	dir := unprotectedTempDir(t)
	fpath := filepath.Join(dir, "deleteme.tmp")
	if err := os.WriteFile(fpath, []byte("delete me"), 0o644); err != nil {
		t.Fatalf("cannot create test file: %v", err)
	}

	size, err := SafeDelete(fpath, false)
	if err != nil {
		t.Fatalf("SafeDelete should delete valid file, got: %v", err)
	}
	if size == 0 {
		t.Error("SafeDelete should return non-zero bytes freed")
	}
	// File must be gone.
	if _, statErr := os.Stat(fpath); !os.IsNotExist(statErr) {
		t.Fatal("file still exists after SafeDelete")
	}
}

func TestSafeDelete_ReturnsCorrectSize(t *testing.T) {
	dir := unprotectedTempDir(t)
	content := strings.Repeat("x", 4096) // exactly 4096 bytes
	fpath := filepath.Join(dir, "sized.tmp")
	if err := os.WriteFile(fpath, []byte(content), 0o644); err != nil {
		t.Fatalf("cannot create test file: %v", err)
	}

	size, err := SafeDelete(fpath, true) // dry run to get size
	if err != nil {
		t.Fatalf("SafeDelete(dryRun=true) error: %v", err)
	}
	if size != int64(len(content)) {
		t.Errorf("expected size %d, got %d", len(content), size)
	}
}

func TestSafeDelete_NonExistentPath(t *testing.T) {
	// Deleting a non-existent file under a safe (non-protected) path
	// should return 0, nil.
	size, err := SafeDelete(`C:\PureWinNonExistent\does\not\exist.tmp`, false)
	if err != nil {
		t.Errorf("SafeDelete on non-existent path should not error, got: %v", err)
	}
	if size != 0 {
		t.Errorf("SafeDelete on non-existent path should return 0, got: %d", size)
	}
}

// ---------------------------------------------------------------------------
// SafeDeleteWithWhitelist tests
// ---------------------------------------------------------------------------

func TestSafeDeleteWithWhitelist_SkipsWhitelisted(t *testing.T) {
	// Whitelist check happens BEFORE ValidatePath, so t.TempDir() is fine.
	dir := t.TempDir()
	fpath := filepath.Join(dir, "whitelisted.tmp")
	if err := os.WriteFile(fpath, []byte("keep me"), 0o644); err != nil {
		t.Fatalf("cannot create test file: %v", err)
	}

	alwaysWhitelisted := func(string) bool { return true }
	_, err := SafeDeleteWithWhitelist(fpath, false, alwaysWhitelisted)
	if err == nil {
		t.Fatal("SafeDeleteWithWhitelist should return error for whitelisted path")
	}
	if !strings.Contains(err.Error(), "whitelisted") {
		t.Errorf("error should mention 'whitelisted', got: %v", err)
	}
	// File must still exist.
	if _, statErr := os.Stat(fpath); os.IsNotExist(statErr) {
		t.Fatal("whitelisted file was deleted — SAFETY VIOLATION")
	}
}

// ---------------------------------------------------------------------------
// FormatSize tests
// ---------------------------------------------------------------------------

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1048576, "1.00 MB"},
		{1572864, "1.50 MB"},
		{1073741824, "1.00 GB"},
		{1099511627776, "1.00 TB"},
	}
	for _, tc := range tests {
		got := FormatSize(tc.bytes)
		if got != tc.expected {
			t.Errorf("FormatSize(%d) = %q, want %q", tc.bytes, got, tc.expected)
		}
	}
}
