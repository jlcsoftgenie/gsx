package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jlcsoftgenie/gsx/internal/ast"
	"github.com/jlcsoftgenie/gsx/internal/compiler"
	"github.com/jlcsoftgenie/gsx/internal/project"
)

const generationCacheSalt = "gsx-generate-cache-v4"

type generateCache struct {
	Version  int                          `json:"version"`
	Packages map[string]generateCacheItem `json:"packages"`
}

type generateCacheItem struct {
	Fingerprint string `json:"fingerprint"`
}

type cacheSet struct {
	caches       map[string]*generateCache
	dirty        map[string]bool
	packageRoots map[string]string
	packages     map[string]map[string]bool
}

func loadCacheSet(roots []string, projectByDir map[string]project.Package) (*cacheSet, error) {
	set := &cacheSet{
		caches:       map[string]*generateCache{},
		dirty:        map[string]bool{},
		packageRoots: map[string]string{},
		packages:     map[string]map[string]bool{},
	}
	for dir, pkg := range projectByDir {
		root := cacheRootForProjectPackage(roots, pkg)
		if root == "" {
			continue
		}
		if _, ok := set.caches[root]; !ok {
			cache, err := readGenerateCache(root)
			if err != nil {
				return nil, err
			}
			set.caches[root] = cache
			set.packages[root] = map[string]bool{}
		}
		set.packageRoots[dir] = root
		set.packages[root][dir] = true
	}
	return set, nil
}

func (s *cacheSet) rootForPackage(pkg *compiler.Package) string {
	if pkg == nil {
		return ""
	}
	return s.packageRoots[pkg.Dir]
}

func (s *cacheSet) shouldSkip(root string, pkg *compiler.Package, fingerprint string) bool {
	cache := s.caches[root]
	if cache == nil || pkg == nil {
		return false
	}
	entry, ok := cache.Packages[pkg.Dir]
	if !ok || entry.Fingerprint != fingerprint {
		return false
	}
	for _, file := range pkg.Files {
		if _, err := os.Stat(generatedPath(file.Path)); err != nil {
			return false
		}
	}
	return true
}

func (s *cacheSet) record(root string, pkg *compiler.Package, fingerprint string) {
	cache := s.caches[root]
	if cache == nil || pkg == nil {
		return
	}
	if cache.Packages == nil {
		cache.Packages = map[string]generateCacheItem{}
	}
	cache.Packages[pkg.Dir] = generateCacheItem{Fingerprint: fingerprint}
	s.dirty[root] = true
}

func (s *cacheSet) save() error {
	for root, cache := range s.caches {
		if cache == nil {
			continue
		}
		pruned := pruneGenerateCache(cache, s.packages[root])
		if pruned {
			s.dirty[root] = true
		}
		if !s.dirty[root] {
			continue
		}
		if err := writeGenerateCache(root, cache); err != nil {
			return err
		}
	}
	return nil
}

func pruneGenerateCache(cache *generateCache, valid map[string]bool) bool {
	if cache == nil || len(cache.Packages) == 0 {
		return false
	}
	changed := false
	for dir := range cache.Packages {
		if valid[dir] {
			continue
		}
		delete(cache.Packages, dir)
		changed = true
	}
	return changed
}

func readGenerateCache(root string) (*generateCache, error) {
	path := generateCachePath(root)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &generateCache{Version: 1, Packages: map[string]generateCacheItem{}}, nil
		}
		return nil, err
	}
	var cache generateCache
	if err := json.Unmarshal(data, &cache); err != nil || cache.Version != 1 {
		return &generateCache{Version: 1, Packages: map[string]generateCacheItem{}}, nil
	}
	if cache.Packages == nil {
		cache.Packages = map[string]generateCacheItem{}
	}
	return &cache, nil
}

func writeGenerateCache(root string, cache *generateCache) error {
	path := generateCachePath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeIfChanged(path, data)
}

func generateCachePath(root string) string {
	return filepath.Join(root, ".gsx", "cache.json")
}

func cacheRootForProjectPackage(roots []string, pkg project.Package) string {
	if pkg.ModuleRoot != "" {
		for _, root := range roots {
			if root == pkg.ModuleRoot || strings.HasPrefix(root, pkg.ModuleRoot+string(filepath.Separator)) {
				return pkg.ModuleRoot
			}
		}
	}
	return longestContainingRoot(roots, pkg.Dir)
}

func longestContainingRoot(roots []string, dir string) string {
	best := ""
	for _, root := range roots {
		if dir != root && !strings.HasPrefix(dir, root+string(filepath.Separator)) {
			continue
		}
		if len(root) > len(best) {
			best = root
		}
	}
	return best
}

func computePackageFingerprints(pkgs []*compiler.Package) map[string]string {
	memo := map[string]string{}
	visiting := map[string]bool{}
	var fingerprint func(*compiler.Package) string
	fingerprint = func(pkg *compiler.Package) string {
		if pkg == nil {
			return ""
		}
		if sum, ok := memo[pkg.Dir]; ok {
			return sum
		}
		if visiting[pkg.Dir] {
			return pkg.Dir
		}
		visiting[pkg.Dir] = true
		h := sha256.New()
		h.Write([]byte(generationCacheSalt))
		h.Write([]byte{0})
		h.Write([]byte(version))
		h.Write([]byte{0})
		h.Write([]byte(pkg.Dir))
		h.Write([]byte{0})
		files := append([]*ast.File(nil), pkg.Files...)
		sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
		for _, file := range files {
			h.Write([]byte(file.Path))
			h.Write([]byte{0})
			h.Write([]byte(file.Source))
			h.Write([]byte{0})
		}
		deps := packageDependencies(pkg)
		sort.Slice(deps, func(i, j int) bool { return deps[i].Dir < deps[j].Dir })
		for _, dep := range deps {
			h.Write([]byte(dep.ImportPath))
			h.Write([]byte{0})
			h.Write([]byte(fingerprint(dep)))
			h.Write([]byte{0})
		}
		sum := hex.EncodeToString(h.Sum(nil))
		delete(visiting, pkg.Dir)
		memo[pkg.Dir] = sum
		return sum
	}
	for _, pkg := range pkgs {
		memo[pkg.Dir] = fingerprint(pkg)
	}
	return memo
}

func packageDependencies(pkg *compiler.Package) []*compiler.Package {
	if pkg == nil || pkg.Registry == nil {
		return nil
	}
	seen := map[string]bool{}
	var deps []*compiler.Package
	for _, file := range pkg.Files {
		for _, imp := range file.Imports {
			dep := pkg.Registry.PackagesByImportPath[imp.Path]
			if dep == nil || dep == pkg || seen[dep.Dir] {
				continue
			}
			seen[dep.Dir] = true
			deps = append(deps, dep)
		}
	}
	return deps
}
