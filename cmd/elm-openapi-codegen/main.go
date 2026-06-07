package main

import (
	"os"

	"github.com/jackevansevo/elm-openapi-codegen/internal/codegen"
)

func main() {
	os.Exit(codegen.RunCLI(os.Args[1:], os.Stderr))
}
