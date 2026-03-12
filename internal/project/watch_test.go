package project

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestWatchDirsSkipsIgnoredDirectories(t *testing.T) {
	tmp := t.TempDir()
	for _, dir := range []string{
		filepath.Join(tmp, "pages"),
		filepath.Join(tmp, "pages", "nested"),
		filepath.Join(tmp, ".gsx"),
		filepath.Join(tmp, ".git"),
		filepath.Join(tmp, "vendor"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	dirs, err := WatchDirs(tmp)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Contains(dirs, tmp) {
		t.Fatalf("expected root directory to be watched")
	}
	if !slices.Contains(dirs, filepath.Join(tmp, "pages")) {
		t.Fatalf("expected nested project directory to be watched")
	}
	if !slices.Contains(dirs, filepath.Join(tmp, "pages", "nested")) {
		t.Fatalf("expected deep project directory to be watched")
	}
	if slices.Contains(dirs, filepath.Join(tmp, ".gsx")) {
		t.Fatalf("did not expect .gsx cache directory to be watched")
	}
	if slices.Contains(dirs, filepath.Join(tmp, ".git")) {
		t.Fatalf("did not expect .git directory to be watched")
	}
	if slices.Contains(dirs, filepath.Join(tmp, "vendor")) {
		t.Fatalf("did not expect vendor directory to be watched")
	}
}
