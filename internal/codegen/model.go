package codegen

type model struct {
	Schemas       map[string]*schemaDef
	SchemaOrder   []string
	ModuleRoot    string
	ModuleByName  map[string]string
	DefsByModule  map[string][]string
	InlineCounter map[string]int
	UsedTypeNames map[string]string
}

type schemaKind int

const (
	kindAlias schemaKind = iota
	kindRecord
	kindEnum
	kindOneOf
	kindAnyOf
)

type schemaDef struct {
	Name               string
	Kind               schemaKind
	Type               typeExpr
	Fields             []fieldDef
	Enum               []enumCase
	Variants           []variantDef
	UnionDiscriminator string
	Recursive          bool
}

type typeKind int

const (
	typeUnknown typeKind = iota
	typeString
	typeInt
	typeFloat
	typeBool
	typeValue
	typeRef
	typeList
	typeDict
)

type typeExpr struct {
	Kind typeKind
	Ref  string
	Elem *typeExpr
}

type fieldDef struct {
	JSONName string
	ElmName  string
	Type     typeExpr
	Required bool
	Nullable bool
	Default  *string
	Exact    *string
}

type enumCase struct {
	Name  string
	Value any
}

type variantDef struct {
	Name          string
	Ref           string
	Discriminator string
}

type elmModule struct {
	Name    string
	Path    string
	Content string
}
