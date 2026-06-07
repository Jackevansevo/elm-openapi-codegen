package codegen

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func writeModules(outDir string, modules []elmModule) error {
	expected := map[string]bool{}
	for _, module := range modules {
		path := filepath.Join(outDir, module.Path)
		expected[filepath.Clean(path)] = true
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(module.Content), 0644); err != nil {
			return err
		}
	}

	if _, err := os.Stat(outDir); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return filepath.WalkDir(outDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".elm" || expected[filepath.Clean(path)] {
			return nil
		}
		generated, err := fileHasGeneratedMarker(path)
		if err != nil {
			return err
		}
		if !generated {
			return nil
		}
		return os.Remove(path)
	})
}

func fileHasGeneratedMarker(path string) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	header := string(content)
	if len(header) > 1024 {
		header = header[:1024]
	}
	return strings.Contains(header, generatedMarker), nil
}
