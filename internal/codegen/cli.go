package codegen

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"io"
)

type options struct {
	specPath   string
	outDir     string
	moduleRoot string
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
	opts := options{moduleRoot: "Generated"}
	flags := flag.NewFlagSet("elm-openapi-codegen", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&opts.outDir, "out", "", "module root output directory")
	flags.StringVar(&opts.moduleRoot, "module-root", opts.moduleRoot, "Elm module root")
	if err := flags.Parse(args); err != nil {
		return opts, usage()
	}
	positionals := flags.Args()
	if len(positionals) != 1 {
		return opts, usage()
	}
	opts.specPath = positionals[0]
	if opts.outDir == "" {
		return opts, errors.New("missing required --out <module-root-dir>")
	}
	if !validModuleName(opts.moduleRoot) {
		return opts, fmt.Errorf("invalid --module-root %q", opts.moduleRoot)
	}
	return opts, nil
}

func usage() error {
	return errors.New("usage: elm-openapi-codegen --out <module-root-dir> [--module-root Generated] <openapi-spec-file>")
}

func run(opts options) error {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(opts.specPath)
	if err != nil {
		return fmt.Errorf("load spec: %w", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		return fmt.Errorf("validate spec: %w", err)
	}

	m, err := buildModel(doc, opts.moduleRoot)
	if err != nil {
		return err
	}
	modules, err := generateModules(m)
	if err != nil {
		return err
	}
	return writeModules(opts.outDir, modules)
}
