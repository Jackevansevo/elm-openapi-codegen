package codegen

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

type options struct {
	specPath string
	outDir   string
}

func RunCLI(args []string, stderr io.Writer) int {
	opts, err := parseOptions(args)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}

	if err := run(opts); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func parseOptions(args []string) (options, error) {
	var opts options
	flags := flag.NewFlagSet("elm-openapi-codegen", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&opts.outDir, "out", "", "generated Elm output directory")
	if err := flags.Parse(args); err != nil {
		return opts, usage()
	}
	positionals := flags.Args()
	if len(positionals) != 1 {
		return opts, usage()
	}
	opts.specPath = positionals[0]
	if opts.outDir == "" {
		return opts, errors.New("missing required --out <elm-output-dir>")
	}
	return opts, nil
}

func usage() error {
	return errors.New("usage: elm-openapi-codegen --out <elm-output-dir> <openapi-spec-file>")
}

func run(opts options) error {
	moduleRoot, err := inferModuleRoot(opts.outDir)
	if err != nil {
		return err
	}

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(opts.specPath)
	if err != nil {
		return fmt.Errorf("load spec: %w", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		return fmt.Errorf("validate spec: %w", err)
	}

	m, err := buildModel(doc, moduleRoot)
	if err != nil {
		return err
	}
	modules, err := generateModules(m)
	if err != nil {
		return err
	}
	return writeModules(opts.outDir, modules)
}

type elmJSON struct {
	SourceDirectories []string `json:"source-directories"`
}

func inferModuleRoot(outDir string) (string, error) {
	absOut, err := filepath.Abs(outDir)
	if err != nil {
		return "", fmt.Errorf("resolve --out: %w", err)
	}
	absOut = filepath.Clean(absOut)

	projectRoot, config, err := findElmProject(absOut)
	if err != nil {
		return "", err
	}

	var matches []string
	for _, sourceDir := range config.SourceDirectories {
		absSourceDir := filepath.Clean(filepath.Join(projectRoot, sourceDir))
		rel, err := filepath.Rel(absSourceDir, absOut)
		if err != nil {
			return "", fmt.Errorf("compare --out with source directory %q: %w", sourceDir, err)
		}
		if rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." && !filepath.IsAbs(rel)) {
			matches = append(matches, rel)
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("--out %q is not inside any source-directories in %s", outDir, filepath.Join(projectRoot, "elm.json"))
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("--out %q matches multiple source-directories in %s", outDir, filepath.Join(projectRoot, "elm.json"))
	}
	return moduleRootFromRelativePath(matches[0])
}

func findElmProject(start string) (string, elmJSON, error) {
	dir := filepath.Clean(start)
	for {
		path := filepath.Join(dir, "elm.json")
		content, err := os.ReadFile(path)
		if err == nil {
			var config elmJSON
			if err := json.Unmarshal(content, &config); err != nil {
				return "", elmJSON{}, fmt.Errorf("parse %s: %w", path, err)
			}
			if len(config.SourceDirectories) == 0 {
				return "", elmJSON{}, fmt.Errorf("%s has no source-directories", path)
			}
			return dir, config, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", elmJSON{}, fmt.Errorf("read %s: %w", path, err)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", elmJSON{}, fmt.Errorf("could not find elm.json for --out %q", start)
		}
		dir = parent
	}
}

func moduleRootFromRelativePath(rel string) (string, error) {
	if rel == "." {
		return "", nil
	}

	parts := strings.Split(filepath.ToSlash(rel), "/")
	for _, part := range parts {
		if !validModuleName(part) {
			return "", fmt.Errorf("--out implies invalid Elm module root %q", strings.Join(parts, "."))
		}
	}
	return strings.Join(parts, "."), nil
}
