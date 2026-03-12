package project

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

func ShouldSkipDir(root, path string) bool {
	base := filepath.Base(path)
	return base == ".git" || base == "vendor" || (strings.HasPrefix(base, ".") && path != root)
}

func WatchDirs(root string) ([]string, error) {
	var dirs []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if ShouldSkipDir(root, path) {
			return filepath.SkipDir
		}
		dirs = append(dirs, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(dirs)
	return dirs, nil
}
