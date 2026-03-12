package project

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jlcsoftgenie/gsx/internal/ast"
	"github.com/jlcsoftgenie/gsx/internal/diagnostics"
	"github.com/jlcsoftgenie/gsx/internal/parser"
)

const TemplateExt = ".gsx"

type Package struct {
	Dir        string
	ImportPath string
	ModulePath string
	ModuleRoot string
	Files      []*ast.File
}

type moduleInfo struct {
	Path string
	Root string
}

func Discover(root string) ([]Package, []diagnostics.Diagnostic, error) {
	byDir := map[string][]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if ShouldSkipDir(root, path) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) == TemplateExt {
			byDir[filepath.Dir(path)] = append(byDir[filepath.Dir(path)], path)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	var dirs []string
	for dir := range byDir {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	cache := map[string]moduleInfo{}
	var pkgs []Package
	var diags []diagnostics.Diagnostic
	for _, dir := range dirs {
		paths := byDir[dir]
		sort.Strings(paths)
		mod, err := findModuleInfo(dir, cache)
		if err != nil {
			return nil, nil, err
		}
		pkg := Package{
			Dir:        dir,
			ImportPath: importPathForDir(mod, dir),
			ModulePath: mod.Path,
			ModuleRoot: mod.Root,
		}
		for _, path := range paths {
			body, err := os.ReadFile(path)
			if err != nil {
				return nil, nil, err
			}
			file, fileDiags := parser.ParseFile(path, string(body))
			pkg.Files = append(pkg.Files, file)
			diags = append(diags, fileDiags...)
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs, diags, nil
}

func importPathForDir(mod moduleInfo, dir string) string {
	if mod.Path == "" || mod.Root == "" {
		return ""
	}
	rel, err := filepath.Rel(mod.Root, dir)
	if err != nil || rel == "." {
		return mod.Path
	}
	return mod.Path + "/" + filepath.ToSlash(rel)
}

func findModuleInfo(dir string, cache map[string]moduleInfo) (moduleInfo, error) {
	if info, ok := cache[dir]; ok {
		return info, nil
	}
	current := dir
	for {
		if info, ok := cache[current]; ok {
			cache[dir] = info
			return info, nil
		}
		gomod := filepath.Join(current, "go.mod")
		if data, err := os.ReadFile(gomod); err == nil {
			info := moduleInfo{Path: parseModulePath(string(data)), Root: current}
			cache[current] = info
			cache[dir] = info
			return info, nil
		} else if err != nil && !os.IsNotExist(err) {
			return moduleInfo{}, err
		}
		parent := filepath.Dir(current)
		if parent == current {
			cache[dir] = moduleInfo{}
			return moduleInfo{}, nil
		}
		current = parent
	}
}

func parseModulePath(text string) string {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		if !strings.HasPrefix(trimmed, "module") {
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
