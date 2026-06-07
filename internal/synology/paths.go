package synology

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func ExpandScanPaths(patterns []string) []string {
	seen := map[string]struct{}{}
	var paths []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil || len(matches) == 0 {
			matches = []string{pattern}
		}
		for _, match := range matches {
			clean := filepath.Clean(match)
			if _, ok := seen[clean]; ok {
				continue
			}
			seen[clean] = struct{}{}
			paths = append(paths, clean)
		}
	}
	sort.Strings(paths)
	return paths
}

func ExistingPaths(patterns []string) []string {
	var existing []string
	for _, path := range ExpandScanPaths(patterns) {
		if _, err := os.Stat(path); err == nil {
			existing = append(existing, path)
		}
	}
	return existing
}

func FindM365LogDBs(patterns []string) []string {
	seen := map[string]struct{}{}
	var candidates []string
	for _, base := range ExpandScanPaths(patterns) {
		info, err := os.Stat(base)
		if err == nil && info.IsDir() {
			candidates = append(candidates, filepath.Join(base, "log.sqlite"))
			continue
		}
		candidates = append(candidates, base)
	}
	return existingUnique(candidates, seen)
}

func FindSQLiteDBs(patterns []string) []string {
	seen := map[string]struct{}{}
	var candidates []string
	for _, root := range ExistingPaths(patterns) {
		info, err := os.Stat(root)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			candidates = append(candidates, root)
			continue
		}
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				name := d.Name()
				if name == "#recycle" || name == "@eaDir" || name == ".snapshot" {
					return filepath.SkipDir
				}
				return nil
			}
			lower := strings.ToLower(d.Name())
			if lower == "activity.db" || strings.HasSuffix(lower, ".db") || strings.HasSuffix(lower, ".sqlite") || strings.HasSuffix(lower, ".sqlite3") {
				candidates = append(candidates, path)
			}
			return nil
		})
	}
	return existingUnique(candidates, seen)
}

func existingUnique(paths []string, seen map[string]struct{}) []string {
	var out []string
	for _, path := range paths {
		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			continue
		}
		if info, err := os.Stat(clean); err == nil && !info.IsDir() {
			seen[clean] = struct{}{}
			out = append(out, clean)
		}
	}
	sort.Strings(out)
	return out
}
