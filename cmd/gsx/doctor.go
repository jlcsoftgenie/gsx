package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/jlcsoftgenie/gsx/internal/codegen"
)

func runDoctor(args []string) error {
	roots, err := collectRoots(args)
	if err != nil {
		return err
	}
	hadIssue := false
	if err := checkGoVersion("1.26.0"); err != nil {
		fmt.Fprintf(os.Stderr, "error go: %v\n", err)
		hadIssue = true
	} else {
		fmt.Fprintln(os.Stderr, "ok go")
	}
	if gopls, ok := findExecutable("gopls"); ok {
		fmt.Fprintf(os.Stderr, "ok gopls: %s\n", gopls)
	} else {
		fmt.Fprintln(os.Stderr, "warn gopls: not found; editor Go intelligence will be limited")
	}

	loaded, diags, err := loadPackages(roots)
	if err != nil {
		return err
	}
	if renderDiagnostics(diags, loaded.filesByPath); hasErrors(diags) {
		return errors.New("doctor found template errors")
	}
	for _, pkg := range loaded.pkgs {
		for _, file := range pkg.Files {
			out, err := codegen.GenerateFile(pkg, file)
			if err != nil {
				return err
			}
			path := generatedPath(file.Path)
			current, err := os.ReadFile(path)
			switch {
			case os.IsNotExist(err):
				fmt.Fprintf(os.Stderr, "error generated file missing: %s\n", path)
				hadIssue = true
			case err != nil:
				return err
			case string(current) != string(out):
				fmt.Fprintf(os.Stderr, "error generated file stale: %s\n", path)
				hadIssue = true
			}
		}
	}
	if hadIssue {
		return errors.New("doctor found issues")
	}
	fmt.Fprintln(os.Stderr, "ok generated files")
	return nil
}

func checkGoVersion(min string) error {
	out, err := exec.Command("go", "env", "GOVERSION").Output()
	if err != nil {
		return err
	}
	got := strings.TrimSpace(string(out))
	if compareGoVersion(got, min) < 0 {
		return fmt.Errorf("need Go %s or newer, found %s", min, got)
	}
	return nil
}

func compareGoVersion(got, min string) int {
	a := parseGoVersion(got)
	b := parseGoVersion(min)
	for i := 0; i < len(a); i++ {
		if a[i] > b[i] {
			return 1
		}
		if a[i] < b[i] {
			return -1
		}
	}
	return 0
}

func parseGoVersion(v string) [3]int {
	v = strings.TrimPrefix(v, "go")
	re := regexp.MustCompile(`\d+`)
	parts := re.FindAllString(v, 3)
	var out [3]int
	for i, part := range parts {
		n, _ := strconv.Atoi(part)
		out[i] = n
	}
	return out
}

func findExecutable(name string) (string, bool) {
	if path, err := exec.LookPath(name); err == nil {
		return path, true
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	for _, candidate := range []string{
		filepath.Join(home, ".local", "bin", name),
		filepath.Join(home, "go", "bin", name),
	} {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return candidate, true
		}
	}
	return "", false
}
