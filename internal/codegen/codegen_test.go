package codegen

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

const referenceFixtureDir = "../../testdata"
const exampleFixtureDir = "../../testdata/examples"

type referenceFixture struct {
	Name       string
	InputPath  string
	ModuleName string
	OutputPath string
}

func TestReferenceFixtures(t *testing.T) {
	for _, fixture := range referenceFixtures(t) {
		fixture := fixture
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()

			doc := loadSpecFile(t, fixture.InputPath)
			m, err := buildModel(doc, "Generated")
			if err != nil {
				t.Fatalf("buildModel returned error: %v", err)
			}
			modules, err := generateModules(m)
			if err != nil {
				t.Fatalf("generateModules returned error: %v", err)
			}
			schemaModules := nonHelperModules(modules)
			if len(schemaModules) != 1 {
				t.Fatalf("expected exactly one non-helper module, got %s", moduleNames(schemaModules))
			}

			module, ok := modulesByName(modules)[fixture.ModuleName]
			if !ok {
				t.Fatalf("expected generated module %s, got %s", fixture.ModuleName, moduleNames(modules))
			}

			if os.Getenv("UPDATE_REFERENCE") != "" {
				if err := os.MkdirAll(filepath.Dir(fixture.OutputPath), 0755); err != nil {
					t.Fatalf("create expected output directory: %v", err)
				}
				if err := os.WriteFile(fixture.OutputPath, []byte(module.Content), 0644); err != nil {
					t.Fatalf("write expected output: %v", err)
				}
			}

			expected, err := os.ReadFile(fixture.OutputPath)
			if err != nil {
				t.Fatalf("read expected output: %v", err)
			}
			if module.Content != string(expected) {
				t.Fatalf("generated output differs from %s", fixture.OutputPath)
			}
		})
	}
}

func referenceFixtures(t *testing.T) []referenceFixture {
	t.Helper()

	var fixtures []referenceFixture
	err := filepath.WalkDir(referenceFixtureDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || entry.Name() != "input.yaml" {
			return nil
		}

		dir := filepath.Dir(path)
		name, err := filepath.Rel(referenceFixtureDir, dir)
		if err != nil {
			return err
		}
		name = filepath.ToSlash(name)
		if strings.HasPrefix(name, "examples/") {
			return nil
		}
		moduleNamePath := filepath.Join(dir, "module.txt")
		moduleNameBytes, err := os.ReadFile(moduleNamePath)
		if err != nil {
			return err
		}
		moduleName := strings.TrimSpace(string(moduleNameBytes))
		if moduleName == "" {
			return errors.New(name + ": module.txt is empty")
		}
		fixtures = append(fixtures, referenceFixture{
			Name:       name,
			InputPath:  path,
			ModuleName: moduleName,
			OutputPath: filepath.Join(dir, "expected", filepath.FromSlash(strings.ReplaceAll(moduleName, ".", "/")+".elm")),
		})
		return nil
	})
	if err != nil {
		t.Fatalf("read reference fixtures: %v", err)
	}

	sort.Slice(fixtures, func(i, j int) bool {
		return fixtures[i].Name < fixtures[j].Name
	})
	return fixtures
}

func TestExampleFixtures(t *testing.T) {
	entries, err := os.ReadDir(exampleFixtureDir)
	if err != nil {
		t.Fatalf("read example fixtures: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "schema-") {
			continue
		}
		dir := filepath.Join(exampleFixtureDir, name)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			doc := loadSpecFile(t, filepath.Join(dir, "input.yaml"))
			m, err := buildModel(doc, "Generated")
			if err != nil {
				t.Fatalf("buildModel returned error: %v", err)
			}
			modules, err := generateModules(m)
			if err != nil {
				t.Fatalf("generateModules returned error: %v", err)
			}

			for _, module := range modules {
				if strings.HasSuffix(module.Name, ".DecodeHelpers") {
					continue
				}
				outputPath := filepath.Join(dir, "expected", filepath.FromSlash(strings.ReplaceAll(module.Name, ".", "/")+".elm"))
				if os.Getenv("UPDATE_EXAMPLES") != "" {
					if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
						t.Fatalf("create expected output directory: %v", err)
					}
					if err := os.WriteFile(outputPath, []byte(module.Content), 0644); err != nil {
						t.Fatalf("write expected output: %v", err)
					}
				}

				expected, err := os.ReadFile(outputPath)
				if err != nil {
					t.Fatalf("read expected output for %s: %v", module.Name, err)
				}
				if module.Content != string(expected) {
					t.Fatalf("generated output for %s differs from %s", module.Name, outputPath)
				}
			}
		})
	}
}

func TestParseOptionsRequiresOut(t *testing.T) {
	_, err := parseOptions([]string{"openapi.yaml"})
	if err == nil {
		t.Fatal("expected parseOptions to reject missing --out")
	}
	if !strings.Contains(err.Error(), "missing required --out <module-root-dir>") {
		t.Fatalf("expected missing --out error, got %v", err)
	}
}

func TestGeneratedModulePathsAreRelativeToModuleRoot(t *testing.T) {
	doc := loadSpecFile(t, filepath.Join(referenceFixtureDir, "object", "primitive-field", "input.yaml"))
	m, err := buildModel(doc, "Generated")
	if err != nil {
		t.Fatalf("buildModel returned error: %v", err)
	}
	modules, err := generateModules(m)
	if err != nil {
		t.Fatalf("generateModules returned error: %v", err)
	}

	byName := modulesByName(modules)
	if _, ok := byName["Generated.DecodeHelpers"]; ok {
		t.Fatal("did not expect helper module to be generated")
	}
	if got, want := byName["Generated.User"].Path, "User.elm"; got != want {
		t.Fatalf("schema module path = %q, want %q", got, want)
	}
}

func TestNameHelpersGenerateASCIIIdentifiers(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "pascal unicode first rune", input: "éclair-status", want: "ClairStatus"},
		{name: "pascal unicode separator", input: "café_com_leite", want: "CafComLeite"},
		{name: "pascal unicode only", input: "用户", want: "Generated"},
		{name: "pascal leading digit", input: "2fa-token", want: "N2faToken"},
		{name: "pascal punctuation only", input: "--", want: "Generated"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pascalName(tt.input); got != tt.want {
				t.Fatalf("pascalName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}

	if got, want := lowerCamelName("éclair-status"), "clairStatus"; got != want {
		t.Fatalf("lowerCamelName(%q) = %q, want %q", "éclair-status", got, want)
	}
}

func TestValidModuleNameUsesASCIIIdentifiers(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "default", in: "Generated", want: true},
		{name: "nested", in: "Generated.Api_V1", want: true},
		{name: "unicode", in: "Generated.Café", want: false},
		{name: "lowercase", in: "generated", want: false},
		{name: "separator", in: "Generated.Api-Client", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validModuleName(tt.in); got != tt.want {
				t.Fatalf("validModuleName(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestOpenAPIUnicodeNameValidation(t *testing.T) {
	valid := loadSpecFileUnvalidated(t, filepath.Join(exampleFixtureDir, "schema-unicode-values", "input.yaml"))
	if err := valid.Validate(context.Background()); err != nil {
		t.Fatalf("expected Unicode property names and enum values to validate: %v", err)
	}

	invalid := loadSpecFileUnvalidated(t, filepath.Join(exampleFixtureDir, "schema-unicode-component-name", "input.yaml"))
	if err := invalid.Validate(context.Background()); err == nil {
		t.Fatal("expected Unicode component schema name to fail validation")
	}
}

func TestBuildModelRejectsSchemaTypeNameCollisions(t *testing.T) {
	doc := loadExampleSpecFile(t, "schema-type-name-collision")
	_, err := buildModel(doc, "Generated")
	if err == nil {
		t.Fatal("expected buildModel to reject colliding generated Elm type names")
	}
	if !strings.Contains(err.Error(), `Elm type "FooBar"`) {
		t.Fatalf("expected Elm type collision error, got %v", err)
	}
}

func TestBuildModelRejectsFieldNameCollisions(t *testing.T) {
	doc := loadExampleSpecFile(t, "schema-field-name-collision")
	_, err := buildModel(doc, "Generated")
	if err == nil {
		t.Fatal("expected buildModel to reject colliding generated Elm field names")
	}
	if !strings.Contains(err.Error(), `Elm field "fooBar"`) {
		t.Fatalf("expected Elm field collision error, got %v", err)
	}
}

func TestInlineNamesAvoidLaterComponentNames(t *testing.T) {
	doc := loadExampleSpecFile(t, "schema-inline-name-avoids-component")
	m, err := buildModel(doc, "Generated")
	if err != nil {
		t.Fatalf("buildModel returned error: %v", err)
	}
	a := m.Schemas["A"]
	if len(a.Fields) != 1 {
		t.Fatalf("expected A to have one field, got %d", len(a.Fields))
	}
	inlineName := a.Fields[0].Type.Ref
	if inlineName == "Z" {
		t.Fatal("inline schema reused later component name Z")
	}
	if got, want := pascalName(inlineName), "Z2"; got != want {
		t.Fatalf("inline field type = %q, want generated Elm type %q", got, want)
	}
	if _, ok := m.Schemas[inlineName]; !ok {
		t.Fatalf("expected inline schema %q to be present", inlineName)
	}
}

func TestOptionalExactStringUsesExactDecoder(t *testing.T) {
	doc := loadExampleSpecFile(t, "schema-optional-exact-string")
	m, err := buildModel(doc, "Generated")
	if err != nil {
		t.Fatalf("buildModel returned error: %v", err)
	}
	modules, err := generateModules(m)
	if err != nil {
		t.Fatalf("generateModules returned error: %v", err)
	}
	pet := modulesByName(modules)["Generated.Pet"]
	if !strings.Contains(pet.Content, `optional "petType" (Decode.map Just (exactString "dog")) Nothing`) {
		t.Fatalf("expected optional exact string decoder, got:\n%s", pet.Content)
	}
}

func TestCleanupMarkerScopeFixture(t *testing.T) {
	dir := filepath.Join(referenceFixtureDir, "cleanup", "marker-scope")
	srcDir := filepath.Join(t.TempDir(), "src")
	outDir := filepath.Join(srcDir, "Generated")
	copyTree(t, filepath.Join(dir, "preexisting", "src"), srcDir)

	doc := loadSpecFile(t, filepath.Join(dir, "input.yaml"))
	m, err := buildModel(doc, "Generated")
	if err != nil {
		t.Fatalf("buildModel returned error: %v", err)
	}
	modules, err := generateModules(m)
	if err != nil {
		t.Fatalf("generateModules returned error: %v", err)
	}
	if err := writeModules(outDir, modules); err != nil {
		t.Fatalf("writeModules returned error: %v", err)
	}

	got := elmFilesUnder(t, srcDir)
	expectedBytes, err := os.ReadFile(filepath.Join(dir, "expected-files.txt"))
	if err != nil {
		t.Fatalf("read expected files: %v", err)
	}
	want := strings.Fields(string(expectedBytes))
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("written files differ\n got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func loadExampleSpecFile(t *testing.T, name string) *openapi3.T {
	t.Helper()
	return loadSpecFile(t, filepath.Join(exampleFixtureDir, name, "input.yaml"))
}

func loadSpecFile(t *testing.T, path string) *openapi3.T {
	t.Helper()
	doc := loadSpecFileUnvalidated(t, path)
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("validate fixture: %v", err)
	}
	return doc
}

func loadSpecFileUnvalidated(t *testing.T, path string) *openapi3.T {
	t.Helper()
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(path)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	return doc
}

func modulesByName(modules []elmModule) map[string]elmModule {
	byName := map[string]elmModule{}
	for _, module := range modules {
		byName[module.Name] = module
	}
	return byName
}

func moduleNames(modules []elmModule) string {
	var names []string
	for _, module := range modules {
		names = append(names, module.Name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func nonHelperModules(modules []elmModule) []elmModule {
	var out []elmModule
	for _, module := range modules {
		if strings.HasSuffix(module.Name, ".DecodeHelpers") {
			continue
		}
		out = append(out, module)
	}
	return out
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0644)
	})
	if err != nil {
		t.Fatalf("copy fixture tree: %v", err)
	}
}

func elmFilesUnder(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".elm" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("list Elm files: %v", err)
	}
	sort.Strings(files)
	return files
}
