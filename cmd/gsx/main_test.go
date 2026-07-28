package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInitCreatesProject(t *testing.T) {
	tmp := t.TempDir()
	if err := run([]string{"init", "--module", "example.com/demo", tmp}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"go.mod", "main.go", "pages.gsx", "pages.gsx.go", ".gitignore"} {
		if _, err := os.Stat(filepath.Join(tmp, name)); err != nil {
			t.Fatalf("expected %s: %v", name, err)
		}
	}
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = tmp
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test failed: %v\n%s", err, out)
	}
}

func TestDoctorDetectsStaleGeneratedFiles(t *testing.T) {
	tmp := t.TempDir()
	if err := run([]string{"init", "--module", "example.com/demo", tmp}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"doctor", tmp}); err != nil {
		t.Fatalf("doctor should pass after init: %v", err)
	}
	path := filepath.Join(tmp, "pages.gsx")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body = append(body, []byte("\ncomponent Extra() { <p>extra</p> }\n")...)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"doctor", tmp}); err == nil {
		t.Fatal("expected doctor to fail when generated files are stale")
	}
}
