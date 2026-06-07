package codegen

import (
	"errors"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"maps"
	"slices"
	"strconv"
)

func buildModel(doc *openapi3.T, moduleRoot string) (*model, error) {
	if doc.Components == nil || len(doc.Components.Schemas) == 0 {
		return nil, errors.New("spec has no components.schemas")
	}

	m := &model{
		Schemas:       map[string]*schemaDef{},
		ModuleRoot:    moduleRoot,
		ModuleByName:  map[string]string{},
		DefsByModule:  map[string][]string{},
		InlineCounter: map[string]int{},
		UsedTypeNames: map[string]string{},
	}

	m.SchemaOrder = slices.Sorted(maps.Keys(doc.Components.Schemas))
	for _, name := range m.SchemaOrder {
		if err := reserveTypeName(m, name, name); err != nil {
			return nil, err
		}
	}

	for _, name := range m.SchemaOrder {
		def, err := normalizeSchema(m, name, doc.Components.Schemas[name], name)
		if err != nil {
			return nil, err
		}
		m.Schemas[name] = def
	}

	graph := dependencyGraph(m)
	components := stronglyConnectedComponents(m.SchemaOrder, graph)
	for _, component := range components {
		slices.Sort(component)
		moduleLeaf := component[0]
		if len(component) == 1 {
			if dependsOn(graph, component[0], component[0]) {
				m.Schemas[component[0]].Recursive = true
			}
		} else {
			for _, name := range component {
				m.Schemas[name].Recursive = true
			}
		}
		moduleName := schemaModuleName(moduleRoot, moduleLeaf)
		for _, name := range component {
			m.ModuleByName[name] = moduleName
			m.DefsByModule[moduleName] = append(m.DefsByModule[moduleName], name)
		}
	}

	for moduleName := range m.DefsByModule {
		slices.Sort(m.DefsByModule[moduleName])
	}

	return m, nil
}

func schemaModuleName(moduleRoot, schemaName string) string {
	leaf := pascalName(schemaName)
	if moduleRoot == "" {
		return leaf
	}
	return moduleRoot + "." + leaf
}

func normalizeSchema(m *model, name string, ref *openapi3.SchemaRef, path string) (*schemaDef, error) {
	if ref == nil || ref.Value == nil {
		return nil, fmt.Errorf("%s: empty schema", path)
	}
	if ref.Ref != "" {
		refName, err := schemaNameFromRef(ref.Ref)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return &schemaDef{Name: name, Kind: kindAlias, Type: typeExpr{Kind: typeRef, Ref: refName}}, nil
	}

	schema := ref.Value
	if len(schema.Enum) > 0 && len(schema.Properties) == 0 && len(schema.OneOf) == 0 && len(schema.AnyOf) == 0 {
		def, err := normalizeEnum(name, schema)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return def, nil
	}
	if len(schema.OneOf) > 0 {
		def, err := normalizeUnion(name, kindOneOf, schema.OneOf, schema.Discriminator)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return def, nil
	}
	if len(schema.AnyOf) > 0 {
		def, err := normalizeUnion(name, kindAnyOf, schema.AnyOf, nil)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return def, nil
	}
	if len(schema.AllOf) > 0 {
		fields, err := flattenAllOf(m, schema.AllOf, name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return &schemaDef{Name: name, Kind: kindRecord, Fields: fields}, nil
	}
	if isMapSchema(schema) {
		t, err := mapType(m, schema, name+"Value")
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return &schemaDef{Name: name, Kind: kindAlias, Type: t}, nil
	}
	if isObjectSchema(schema) {
		fields, err := normalizeFields(m, schema, name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return &schemaDef{Name: name, Kind: kindRecord, Fields: fields}, nil
	}

	t, err := normalizeType(m, ref, name)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &schemaDef{Name: name, Kind: kindAlias, Type: t}, nil
}

func normalizeEnum(name string, schema *openapi3.Schema) (*schemaDef, error) {
	def := &schemaDef{Name: name, Kind: kindEnum}
	seen := map[string]int{}
	for _, value := range schema.Enum {
		if !isSupportedEnumValue(value) {
			return nil, fmt.Errorf("enum value %v has unsupported type %T", value, value)
		}
		caseName := enumCaseName(name, value)
		if seen[caseName] > 0 {
			seen[caseName]++
			caseName += strconv.Itoa(seen[caseName])
		} else {
			seen[caseName] = 1
		}
		def.Enum = append(def.Enum, enumCase{Name: caseName, Value: value})
	}
	return def, nil
}

func normalizeUnion(name string, kind schemaKind, refs openapi3.SchemaRefs, discriminator *openapi3.Discriminator) (*schemaDef, error) {
	def := &schemaDef{Name: name, Kind: kind}
	mappingByRef := map[string]string{}
	if discriminator != nil {
		def.UnionDiscriminator = discriminator.PropertyName
		for value, ref := range discriminator.Mapping {
			refName, err := schemaNameFromRef(ref.Ref)
			if err != nil {
				return nil, fmt.Errorf("discriminator mapping %q: %w", value, err)
			}
			mappingByRef[refName] = value
		}
	}
	for _, ref := range refs {
		refName, err := schemaNameFromRef(ref.Ref)
		if err != nil {
			return nil, err
		}
		var discriminatorValue string
		if discriminator != nil {
			discriminatorValue = mappingByRef[refName]
			if discriminatorValue == "" {
				discriminatorValue = lowerCamelName(refName)
			}
		}
		def.Variants = append(def.Variants, variantDef{
			Name:          pascalName(name) + pascalName(refName),
			Ref:           refName,
			Discriminator: discriminatorValue,
		})
	}
	return def, nil
}

func flattenAllOf(m *model, refs openapi3.SchemaRefs, parent string) ([]fieldDef, error) {
	var fields []fieldDef
	for _, ref := range refs {
		if ref.Ref != "" {
			refName, err := schemaNameFromRef(ref.Ref)
			if err != nil {
				return nil, err
			}
			target, ok := m.Schemas[refName]
			if !ok {
				targetRef := ref
				if ref.Value != nil {
					targetRef = &openapi3.SchemaRef{Value: ref.Value}
				}
				target, err = normalizeSchema(m, refName, targetRef, refName)
				if err != nil {
					return nil, err
				}
				m.Schemas[refName] = target
			}
			if target.Kind != kindRecord {
				return nil, fmt.Errorf("allOf reference %s is not an object schema", refName)
			}
			fields = append(fields, target.Fields...)
			continue
		}
		if ref.Value == nil {
			continue
		}
		inline, err := normalizeFields(m, ref.Value, parent)
		if err != nil {
			return nil, err
		}
		fields = append(fields, inline...)
	}
	return fields, nil
}

func normalizeFields(m *model, schema *openapi3.Schema, parent string) ([]fieldDef, error) {
	required := make(map[string]bool, len(schema.Required))
	for _, name := range schema.Required {
		required[name] = true
	}

	names := slices.Sorted(maps.Keys(schema.Properties))
	fieldNames := map[string]string{}

	var fields []fieldDef
	for _, jsonName := range names {
		prop := schema.Properties[jsonName]
		fieldName := safeLowerCamel(jsonName)
		if previous, ok := fieldNames[fieldName]; ok {
			return nil, fmt.Errorf("%s: properties %q and %q both generate Elm field %q", parent, previous, jsonName, fieldName)
		}
		fieldNames[fieldName] = jsonName
		t, err := normalizeFieldType(m, prop, parent, jsonName)
		if err != nil {
			return nil, err
		}
		field := fieldDef{
			JSONName: jsonName,
			ElmName:  fieldName,
			Type:     t,
			Required: required[jsonName],
		}
		if prop.Value != nil {
			field.Nullable = prop.Value.Nullable || hasNullType(prop.Value)
			if len(prop.Value.Enum) == 1 && typeName(prop.Value) == "string" {
				if s, ok := prop.Value.Enum[0].(string); ok {
					field.Exact = &s
				}
			}
			if prop.Value.Default != nil {
				if field.Nullable {
					return nil, fmt.Errorf("%s.%s: defaults for nullable fields are unsupported", parent, jsonName)
				}
				defaultValue, err := elmDefault(t, prop.Value.Default)
				if err != nil {
					return nil, fmt.Errorf("%s.%s: %w", parent, jsonName, err)
				}
				field.Default = &defaultValue
			}
		}
		fields = append(fields, field)
	}
	return fields, nil
}

func normalizeFieldType(m *model, ref *openapi3.SchemaRef, parent, field string) (typeExpr, error) {
	if ref == nil {
		return typeExpr{Kind: typeValue}, nil
	}
	if ref.Ref != "" {
		name, err := schemaNameFromRef(ref.Ref)
		return typeExpr{Kind: typeRef, Ref: name}, err
	}
	if ref.Value == nil {
		return typeExpr{Kind: typeValue}, nil
	}
	if isInlineObject(ref.Value) {
		inlineName := uniqueInlineName(m, field)
		def, err := normalizeSchema(m, inlineName, ref, inlineName)
		if err != nil {
			return typeExpr{}, err
		}
		m.Schemas[inlineName] = def
		m.SchemaOrder = append(m.SchemaOrder, inlineName)
		return typeExpr{Kind: typeRef, Ref: inlineName}, nil
	}
	return normalizeType(m, ref, pascalName(field))
}

func normalizeType(m *model, ref *openapi3.SchemaRef, parent string) (typeExpr, error) {
	if ref == nil || ref.Value == nil {
		return typeExpr{Kind: typeValue}, nil
	}
	if ref.Ref != "" {
		name, err := schemaNameFromRef(ref.Ref)
		return typeExpr{Kind: typeRef, Ref: name}, err
	}
	schema := ref.Value
	switch typeName(schema) {
	case "string":
		return typeExpr{Kind: typeString}, nil
	case "integer":
		return typeExpr{Kind: typeInt}, nil
	case "number":
		return typeExpr{Kind: typeFloat}, nil
	case "boolean":
		return typeExpr{Kind: typeBool}, nil
	case "array":
		item, err := normalizeArrayItemType(m, schema.Items, parent+"Item")
		if err != nil {
			return typeExpr{}, err
		}
		return typeExpr{Kind: typeList, Elem: &item}, nil
	case "object":
		if isMapSchema(schema) {
			return mapType(m, schema, parent+"Value")
		}
		if len(schema.Properties) > 0 {
			inlineName := uniqueInlineName(m, parent)
			def, err := normalizeSchema(m, inlineName, ref, inlineName)
			if err != nil {
				return typeExpr{}, err
			}
			m.Schemas[inlineName] = def
			m.SchemaOrder = append(m.SchemaOrder, inlineName)
			return typeExpr{Kind: typeRef, Ref: inlineName}, nil
		}
		return typeExpr{Kind: typeValue}, nil
	default:
		if len(schema.Properties) > 0 {
			inlineName := uniqueInlineName(m, parent)
			def, err := normalizeSchema(m, inlineName, ref, inlineName)
			if err != nil {
				return typeExpr{}, err
			}
			m.Schemas[inlineName] = def
			m.SchemaOrder = append(m.SchemaOrder, inlineName)
			return typeExpr{Kind: typeRef, Ref: inlineName}, nil
		}
		return typeExpr{Kind: typeValue}, nil
	}
}

func normalizeArrayItemType(m *model, ref *openapi3.SchemaRef, parent string) (typeExpr, error) {
	if ref != nil && ref.Ref != "" {
		name, err := schemaNameFromRef(ref.Ref)
		return typeExpr{Kind: typeRef, Ref: name}, err
	}
	if ref != nil && ref.Value != nil && isInlineObject(ref.Value) {
		inlineName := uniqueInlineName(m, parent)
		def, err := normalizeSchema(m, inlineName, ref, inlineName)
		if err != nil {
			return typeExpr{}, err
		}
		m.Schemas[inlineName] = def
		m.SchemaOrder = append(m.SchemaOrder, inlineName)
		return typeExpr{Kind: typeRef, Ref: inlineName}, nil
	}
	return normalizeType(m, ref, parent)
}

func mapType(m *model, schema *openapi3.Schema, parent string) (typeExpr, error) {
	if schema.AdditionalProperties.Schema != nil {
		elem, err := normalizeType(m, schema.AdditionalProperties.Schema, parent)
		if err != nil {
			return typeExpr{}, err
		}
		return typeExpr{Kind: typeDict, Elem: &elem}, nil
	}
	return typeExpr{Kind: typeDict, Elem: &typeExpr{Kind: typeValue}}, nil
}

func isObjectSchema(schema *openapi3.Schema) bool {
	return typeName(schema) == "object" || len(schema.Properties) > 0
}

func isInlineObject(schema *openapi3.Schema) bool {
	return isObjectSchema(schema) && len(schema.Properties) > 0 && !isMapSchema(schema)
}

func isMapSchema(schema *openapi3.Schema) bool {
	return len(schema.Properties) == 0 && (schema.AdditionalProperties.Has != nil && *schema.AdditionalProperties.Has || schema.AdditionalProperties.Schema != nil)
}

func typeName(schema *openapi3.Schema) string {
	if schema == nil || schema.Type == nil {
		return ""
	}
	for _, typ := range schema.Type.Slice() {
		if typ != "null" {
			return typ
		}
	}
	return ""
}

func hasNullType(schema *openapi3.Schema) bool {
	if schema == nil || schema.Type == nil {
		return false
	}
	return slices.Contains(schema.Type.Slice(), "null")
}
