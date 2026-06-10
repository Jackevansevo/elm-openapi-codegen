package codegen

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
)

func generateModules(m *model) ([]elmModule, error) {
	var modules []elmModule

	moduleNames := slices.Sorted(maps.Keys(m.DefsByModule))
	for _, moduleName := range moduleNames {
		content, err := generateSchemaModule(m, moduleName, m.DefsByModule[moduleName])
		if err != nil {
			return nil, err
		}
		modules = append(modules, elmModule{
			Name:    moduleName,
			Path:    elmModulePath(m.ModuleRoot, moduleName),
			Content: content,
		})
	}
	return modules, nil
}

func elmModulePath(moduleRoot, name string) string {
	relativeName := strings.TrimPrefix(name, moduleRoot+".")
	if relativeName == name {
		relativeName = strings.TrimPrefix(name, moduleRoot)
	}
	return filepath.Join(strings.Split(relativeName, ".")...) + ".elm"
}

func generateSchemaModule(m *model, moduleName string, names []string) (string, error) {
	exposures := moduleExposures(m, names)
	imports := moduleImports(m, names)

	var decls []elmDecl
	for _, name := range names {
		schemaDecls, err := schemaToElmDecls(m.Schemas[name])
		if err != nil {
			return "", err
		}
		decls = append(decls, schemaDecls...)
	}

	var trailing []elmDecl
	for _, exactName := range exactFunctionNamesForModule(m, names) {
		trailing = append(trailing, exactValueDecl(exactName))
	}

	file := elmFile{
		Name:     moduleName,
		Exposing: exposures,
		Imports:  imports,
		Decls:    decls,
		Trailing: trailing,
	}
	return file.render(), nil
}

func moduleExposures(m *model, names []string) []string {
	var out []string
	for _, name := range names {
		def := m.Schemas[name]
		out = append(out, exposedTypeName(name, def))
		out = append(out, decoderName(name))
		if def.Kind == kindEnum {
			out = append(out, encoderName(name))
			out = append(out, "all")
			out = append(out, "toString")
			out = append(out, "fromString")
		}
	}
	slices.Sort(out)
	return out
}

func moduleImports(m *model, names []string) []string {
	importSet := map[string]string{}
	needsDict := false
	needsOptional := false
	needsPipeline := false
	needsEncode := false

	local := make(map[string]bool, len(names))
	for _, name := range names {
		local[name] = true
	}
	for _, name := range names {
		def := m.Schemas[name]
		switch def.Kind {
		case kindRecord:
			for _, field := range def.Fields {
				if field.Required {
					needsPipeline = true
				} else {
					needsOptional = true
				}
				if typeNeedsDict(field.Type) {
					needsDict = true
				}
				for _, dep := range typeRefs(field.Type) {
					if !local[dep] {
						addSchemaImport(importSet, m, dep)
					}
				}
			}
		case kindAlias:
			if typeNeedsDict(def.Type) {
				needsDict = true
			}
			for _, dep := range typeRefs(def.Type) {
				if !local[dep] {
					addSchemaImport(importSet, m, dep)
				}
			}
		case kindEnum:
			needsEncode = true
		case kindOneOf, kindAnyOf:
			for _, variant := range def.Variants {
				if !local[variant.Ref] {
					addSchemaImport(importSet, m, variant.Ref)
				}
			}
		}
	}

	var imports []string
	if needsDict {
		imports = append(imports, "import Dict exposing (Dict)")
	}
	imports = append(imports, "import Json.Decode as Decode exposing (Decoder)")
	if needsEncode {
		imports = append(imports, "import Json.Encode as Encode")
	}
	if needsOptional || needsPipeline {
		var exposing []string
		if needsOptional {
			exposing = append(exposing, "optional")
		}
		if needsPipeline {
			exposing = append(exposing, "required")
		}
		imports = append(imports, "import Json.Decode.Pipeline exposing ("+strings.Join(exposing, ", ")+")")
	}
	for _, imp := range importSet {
		imports = append(imports, imp)
	}
	slices.Sort(imports)
	return imports
}

func addSchemaImport(importSet map[string]string, m *model, dep string) {
	moduleName := m.ModuleByName[dep]
	if moduleName == "" {
		return
	}
	var exposing []string
	for _, name := range m.DefsByModule[moduleName] {
		exposing = append(exposing, exposedTypeName(name, m.Schemas[name]))
		exposing = append(exposing, decoderName(name))
	}
	slices.Sort(exposing)
	importSet[moduleName] = fmt.Sprintf("import %s exposing (%s)", moduleName, strings.Join(exposing, ", "))
}

func exposedTypeName(name string, def *schemaDef) string {
	if def.Kind == kindEnum || def.Kind == kindOneOf || def.Kind == kindAnyOf {
		return pascalName(name) + "(..)"
	}
	return pascalName(name)
}
