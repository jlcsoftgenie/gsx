package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/jlcsoftgenie/gsx/internal/project"
)

func watchRoots(roots []string, debounce time.Duration, build bool, runCycle func([]string) error, command string) error {
	if err := runWatchCycle(roots, runCycle, command); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	watched := map[string]bool{}
	for _, root := range roots {
		if err := addWatchTree(watcher, watched, root); err != nil {
			return err
		}
	}

	var timer *time.Timer
	var timerCh <-chan time.Time
	schedule := func() {
		if timer == nil {
			timer = time.NewTimer(debounce)
			timerCh = timer.C
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(debounce)
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Op&(fsnotify.Create|fsnotify.Rename) != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					parent := rootForPath(roots, filepath.Dir(event.Name))
					if parent != "" && !project.ShouldSkipDir(parent, event.Name) {
						if err := addWatchTree(watcher, watched, event.Name); err != nil {
							fmt.Fprintln(os.Stderr, err)
						}
					}
				}
			}
			if !watchEventRelevant(event.Name, build) {
				continue
			}
			if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
				delete(watched, event.Name)
			}
			fmt.Fprintln(os.Stderr, "gsx: change detected")
			schedule()
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintln(os.Stderr, err)
		case <-timerCh:
			if err := runWatchCycle(roots, runCycle, command); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			timerCh = nil
			timer = nil
		}
	}
}

func runWatchCycle(roots []string, runCycle func([]string) error, command string) error {
	if err := runCycle(roots); err != nil {
		return err
	}
	if command == "" {
		return nil
	}
	return runShellCommand(roots[0], command)
}

func addWatchTree(watcher *fsnotify.Watcher, watched map[string]bool, root string) error {
	dirs, err := project.WatchDirs(root)
	if err != nil {
		return err
	}
	for _, dir := range dirs {
		if watched[dir] {
			continue
		}
		if err := watcher.Add(dir); err != nil {
			return err
		}
		watched[dir] = true
	}
	return nil
}

func watchEventRelevant(path string, build bool) bool {
	base := filepath.Base(path)
	switch filepath.Ext(path) {
	case project.TemplateExt:
		return true
	case ".go":
		return build
	}
	if !build {
		return false
	}
	return base == "go.mod" || base == "go.sum" || base == "go.work" || base == "go.work.sum"
}

func rootForPath(roots []string, path string) string {
	best := ""
	for _, root := range roots {
		if path != root && !hasPathPrefix(path, root) {
			continue
		}
		if len(root) > len(best) {
			best = root
		}
	}
	return best
}

func hasPathPrefix(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+string(filepath.Separator))
}
