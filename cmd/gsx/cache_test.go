package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestComputePackageFingerprintsTrackImportedPackages(t *testing.T) {
	tmp := t.TempDir()
	writeTestFile(t, filepath.Join(tmp, "go.mod"), "module example.com/demo\n\ngo 1.23.0\n")
	writeTestFile(t, filepath.Join(tmp, "shared", "layout.gsx"), `package shared

component Layout(title string) {
  <html>
    <body>
      <slot />
    </body>
  </html>
}
`)
	writeTestFile(t, filepath.Join(tmp, "app", "page.gsx"), `package app

import shared "example.com/demo/shared"

component Page(title string) {
  <shared.Layout title={title}>
    <p>{title}</p>
  </shared.Layout>
}
`)
	writeTestFile(t, filepath.Join(tmp, "other", "page.gsx"), `package other

component Page(title string) {
  <p>{title}</p>
}
`)

	loaded, _, err := loadPackages([]string{tmp})
	if err != nil {
		t.Fatal(err)
	}
	before := computePackageFingerprints(loaded.pkgs)

	writeTestFile(t, filepath.Join(tmp, "shared", "layout.gsx"), `package shared

component Layout(title string) {
  <html>
    <head>
      <title>{title}</title>
    </head>
    <body>
      <slot />
    </body>
  </html>
}
`)

	loaded, _, err = loadPackages([]string{tmp})
	if err != nil {
		t.Fatal(err)
	}
	after := computePackageFingerprints(loaded.pkgs)

	appDir := filepath.Join(tmp, "app")
	sharedDir := filepath.Join(tmp, "shared")
	otherDir := filepath.Join(tmp, "other")
	if before[sharedDir] == after[sharedDir] {
		t.Fatalf("expected shared package fingerprint to change")
	}
	if before[appDir] == after[appDir] {
		t.Fatalf("expected importing package fingerprint to change")
	}
	if before[otherDir] != after[otherDir] {
		t.Fatalf("expected unrelated package fingerprint to stay stable")
	}
}

func TestRunGenerateWritesCache(t *testing.T) {
	tmp := t.TempDir()
	writeTestFile(t, filepath.Join(tmp, "go.mod"), "module example.com/demo\n\ngo 1.23.0\n")
	writeTestFile(t, filepath.Join(tmp, "pages.gsx"), `package main

component HomePage(title string) {
  <h1>{title}</h1>
}
`)

	if err := runGenerate([]string{tmp}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, ".gsx", "cache.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cache generateCache
	if err := json.Unmarshal(data, &cache); err != nil {
		t.Fatal(err)
	}
	if len(cache.Packages) != 1 {
		t.Fatalf("expected one cached package, got %d", len(cache.Packages))
	}
}

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
