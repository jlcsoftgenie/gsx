package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strings"
	"time"

	"github.com/jlcsoftgenie/gsx/internal/ast"
	"github.com/jlcsoftgenie/gsx/internal/codegen"
	"github.com/jlcsoftgenie/gsx/internal/compiler"
	"github.com/jlcsoftgenie/gsx/internal/diagnostics"
	"github.com/jlcsoftgenie/gsx/internal/formatter"
	"github.com/jlcsoftgenie/gsx/internal/linter"
	"github.com/jlcsoftgenie/gsx/internal/project"
)

const (
	version    = "0.4.1"
	modulePath = "github.com/jlcsoftgenie/gsx"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usageError()
	}
	switch args[0] {
	case "generate":
		return runGenerate(args[1:])
	case "build":
		return runBuild(args[1:])
	case "check":
		return runCheck(args[1:])
	case "lint":
		return runLint(args[1:])
	case "fmt":
		return runFmt(args[1:])
	case "watch":
		return runWatch(args[1:])
	case "doctor":
		return runDoctor(args[1:])
	case "init":
		return runInit(args[1:])
	case "version":
		fmt.Println(version)
		return nil
	default:
		return usageError()
	}
}

func usageError() error {
	return errors.New("usage: gsx <generate|build|check|lint|fmt|watch|doctor|init|version> [paths...]")
}

func runGenerate(args []string) error {
	roots, err := collectRoots(args)
	if err != nil {
		return err
	}
	loaded, diags, err := loadPackages(roots)
	if err != nil {
		return err
	}
	if renderDiagnostics(diags, loaded.filesByPath); hasErrors(diags) {
		return errors.New("generation failed")
	}
	cacheSet, err := loadCacheSet(roots, loaded.projectByDir)
	if err != nil {
		return err
	}
	fingerprints := computePackageFingerprints(loaded.pkgs)
	for _, pkg := range loaded.pkgs {
		root := cacheSet.rootForPackage(pkg)
		if root != "" && cacheSet.shouldSkip(root, pkg, fingerprints[pkg.Dir]) {
			continue
		}
		for _, file := range pkg.Files {
			out, err := codegen.GenerateFile(pkg, file)
			if err != nil {
				return err
			}
			genPath := generatedPath(file.Path)
			if err := writeIfChanged(genPath, out); err != nil {
				return err
			}
		}
		if root != "" {
			cacheSet.record(root, pkg, fingerprints[pkg.Dir])
		}
	}
	if err := cacheSet.save(); err != nil {
		return err
	}
	return nil
}

func runBuild(args []string) error {
	roots, err := collectRoots(args)
	if err != nil {
		return err
	}
	if err := runGenerate(roots); err != nil {
		return err
	}
	for _, root := range roots {
		cmd := exec.Command("go", "build", "./...")
		cmd.Dir = root
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

func runCheck(args []string) error {
	roots, err := collectRoots(args)
	if err != nil {
		return err
	}
	loaded, diags, err := loadPackages(roots)
	if err != nil {
		return err
	}
	for _, pkg := range loaded.pkgs {
		diags = append(diags, linter.LintPackage(pkg, true)...)
	}
	had := renderDiagnostics(diags, loaded.filesByPath)
	if had || hasWarnings(diags) {
		return errors.New("check failed")
	}
	return nil
}

func runLint(args []string) error {
	roots, err := collectRoots(args)
	if err != nil {
		return err
	}
	loaded, diags, err := loadPackages(roots)
	if err != nil {
		return err
	}
	for _, pkg := range loaded.pkgs {
		diags = append(diags, linter.LintPackage(pkg, false)...)
	}
	had := renderDiagnostics(diags, loaded.filesByPath)
	if had || hasWarnings(diags) {
		return errors.New("lint failed")
	}
	return nil
}

func runFmt(args []string) error {
	fs := flag.NewFlagSet("fmt", flag.ContinueOnError)
	write := fs.Bool("write", false, "write result to file")
	check := fs.Bool("check", false, "check formatting")
	if err := fs.Parse(args); err != nil {
		return err
	}
	paths := fs.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}
	files, err := collectTemplateFiles(paths)
	if err != nil {
		return err
	}
	changed := false
	for _, path := range files {
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out, err := formatter.Format(path, string(body))
		if err != nil {
			return err
		}
		if out == string(body) {
			continue
		}
		changed = true
		if *check {
			fmt.Fprintln(os.Stderr, path)
			continue
		}
		if *write || !*check {
			if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
				return err
			}
		}
	}
	if *check && changed {
		return errors.New("format check failed")
	}
	return nil
}

func runWatch(args []string) error {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	interval := fs.Duration("interval", 250*time.Millisecond, "event debounce interval")
	build := fs.Bool("build", false, "run build instead of generate")
	command := fs.String("command", "", "shell command to run after a successful cycle")
	if err := fs.Parse(args); err != nil {
		return err
	}
	paths := fs.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}
	roots, err := collectRoots(paths)
	if err != nil {
		return err
	}
	runCycle := runGenerate
	if *build {
		runCycle = runBuild
	}
	return watchRoots(roots, *interval, *build, runCycle, *command)
}

func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	module := fs.String("module", "", "module path to create when go.mod is missing")
	pkgName := fs.String("package", "main", "package name for starter files")
	force := fs.Bool("force", false, "overwrite starter files if they already exist")
	if err := fs.Parse(args); err != nil {
		return err
	}
	target := "."
	if fs.NArg() > 0 {
		target = fs.Arg(0)
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return err
	}
	files, err := initFiles(abs, *module, *pkgName)
	if err != nil {
		return err
	}
	for path, body := range files {
		if !*force {
			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("refusing to overwrite %s without --force", path)
			}
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return err
		}
	}
	if err := runGenerate([]string{abs}); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "initialized GSX project in %s\n", abs)
	return nil
}

type loadedPackages struct {
	pkgs         []*compiler.Package
	filesByPath  map[string]*ast.File
	projectByDir map[string]project.Package
}

func loadPackages(roots []string) (*loadedPackages, []diagnostics.Diagnostic, error) {
	all := &loadedPackages{filesByPath: map[string]*ast.File{}, projectByDir: map[string]project.Package{}}
	registry := compiler.NewRegistry()
	var diags []diagnostics.Diagnostic
	seenDirs := map[string]bool{}
	var discovered []project.Package
	for _, root := range roots {
		projectPkgs, parseDiags, err := project.Discover(root)
		if err != nil {
			return nil, nil, err
		}
		diags = append(diags, parseDiags...)
		for _, projectPkg := range projectPkgs {
			if seenDirs[projectPkg.Dir] {
				continue
			}
			seenDirs[projectPkg.Dir] = true
			all.projectByDir[projectPkg.Dir] = projectPkg
			for _, file := range projectPkg.Files {
				all.filesByPath[file.Path] = file
			}
			discovered = append(discovered, projectPkg)
		}
	}
	sort.Slice(discovered, func(i, j int) bool { return discovered[i].Dir < discovered[j].Dir })
	for _, projectPkg := range discovered {
		pkg, buildDiags := compiler.BuildPackage(projectPkg.Dir, projectPkg.ImportPath, projectPkg.Files)
		diags = append(diags, buildDiags...)
		registry.Add(pkg)
		all.pkgs = append(all.pkgs, pkg)
	}
	diags = append(diags, compiler.ValidateAll(registry)...)
	return all, diags, nil
}

func collectRoots(args []string) ([]string, error) {
	if len(args) == 0 {
		args = []string{"."}
	}
	seen := map[string]bool{}
	var roots []string
	for _, arg := range args {
		root := arg
		info, err := os.Stat(arg)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			root = filepath.Dir(arg)
		}
		abs, err := filepath.Abs(root)
		if err != nil {
			return nil, err
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		roots = append(roots, abs)
	}
	sort.Strings(roots)
	return roots, nil
}

func collectTemplateFiles(paths []string) ([]string, error) {
	roots, err := collectRoots(paths)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, root := range roots {
		projectPkgs, _, err := project.Discover(root)
		if err != nil {
			return nil, err
		}
		for _, pkg := range projectPkgs {
			for _, file := range pkg.Files {
				files = append(files, file.Path)
			}
		}
	}
	sort.Strings(files)
	return files, nil
}

func generatedPath(path string) string {
	return strings.TrimSuffix(path, project.TemplateExt) + ".gsx.go"
}

func writeIfChanged(path string, body []byte) error {
	current, err := os.ReadFile(path)
	if err == nil && string(current) == string(body) {
		return nil
	}
	return os.WriteFile(path, body, 0o644)
}

func renderDiagnostics(diags []diagnostics.Diagnostic, files map[string]*ast.File) bool {
	sort.Slice(diags, func(i, j int) bool {
		if diags[i].Span.File == diags[j].Span.File {
			if diags[i].Span.Start.Line == diags[j].Span.Start.Line {
				return diags[i].Span.Start.Column < diags[j].Span.Start.Column
			}
			return diags[i].Span.Start.Line < diags[j].Span.Start.Line
		}
		return diags[i].Span.File < diags[j].Span.File
	})
	hadErrors := false
	for _, diag := range diags {
		if diag.Severity == diagnostics.SeverityError {
			hadErrors = true
		}
		var src *diagnostics.Source
		if file, ok := files[diag.Span.File]; ok {
			src = diagnostics.NewSource(file.Path, file.Source)
		}
		fmt.Fprintln(os.Stderr, diagnostics.Render(diag, src))
	}
	return hadErrors
}

func hasErrors(diags []diagnostics.Diagnostic) bool {
	for _, diag := range diags {
		if diag.Severity == diagnostics.SeverityError {
			return true
		}
	}
	return false
}

func hasWarnings(diags []diagnostics.Diagnostic) bool {
	for _, diag := range diags {
		if diag.Severity == diagnostics.SeverityWarning {
			return true
		}
	}
	return false
}

func runShellCommand(dir, command string) error {
	cmd := exec.Command("bash", "-lc", command)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func initFiles(targetDir, modulePath, packageName string) (map[string]string, error) {
	result := map[string]string{}
	if hasMod, err := hasGoMod(targetDir); err != nil {
		return nil, err
	} else if !hasMod {
		if modulePath == "" {
			modulePath = "example.com/gsx-app"
		}
		result[filepath.Join(targetDir, "go.mod")] = buildInitGoMod(modulePath)
	}
	result[filepath.Join(targetDir, "main.go")] = fmt.Sprintf(`package %s

import (
	"log"
	"net/http"
)

//go:generate go run %s/cmd/gsx generate .

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := RenderHomePage(w, "GSX Starter"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
`, packageName, modulePath)
	result[filepath.Join(targetDir, "pages.gsx")] = fmt.Sprintf(`package %s

component Layout(title string) {
  <!doctype html>
  <html>
    <head>
      <meta charset="utf-8" />
      <meta name="viewport" content="width=device-width, initial-scale=1" />
      <title>{title}</title>
    </head>
    <body>
      <slot />
    </body>
  </html>
}

component HomePage(title string) {
  <Layout title={title}>
    <main>
      <h1>{title}</h1>
      <p>Edit <code>pages.gsx</code> and rerun <code>gsx generate</code>.</p>
    </main>
  </Layout>
}
`, packageName)
	result[filepath.Join(targetDir, ".gitignore")] = "*.gsx.go\n"
	return result, nil
}

func hasGoMod(dir string) (bool, error) {
	current := dir
	for {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return true, nil
		} else if err != nil && !os.IsNotExist(err) {
			return false, err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false, nil
		}
		current = parent
	}
}

func buildInitGoMod(appModulePath string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "module %s\n\ngo 1.26.0\n", appModulePath)
	if replaceRoot, ok := localGSXReplaceRoot(); ok {
		fmt.Fprintf(&b, "\nrequire %s v0.0.0\n\nreplace %s => %s\n", modulePath, modulePath, filepath.ToSlash(replaceRoot))
	}
	return b.String()
}

func localGSXReplaceRoot() (string, bool) {
	var candidates []string
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, cwd)
	}
	if _, file, _, ok := goruntime.Caller(0); ok {
		candidates = append(candidates, filepath.Dir(file))
	}
	seen := map[string]bool{}
	for _, start := range candidates {
		current := start
		for {
			if seen[current] {
				break
			}
			seen[current] = true
			data, err := os.ReadFile(filepath.Join(current, "go.mod"))
			if err == nil {
				if parseModulePath(string(data)) == modulePath {
					return current, true
				}
			} else if !os.IsNotExist(err) {
				return "", false
			}
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
			current = parent
		}
	}
	return "", false
}

func parseModulePath(text string) string {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || !strings.HasPrefix(trimmed, "module") {
			continue
		}
		parts := strings.Fields(trimmed)
		if len(parts) < 2 {
			return ""
		}
		return strings.Trim(parts[1], "`\"")
	}
	return ""
}
