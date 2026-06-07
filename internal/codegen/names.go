package codegen

import (
	"fmt"
	"strconv"
	"strings"
)

var elmReserved = map[string]bool{
	"as": true, "case": true, "else": true, "exposing": true, "if": true,
	"import": true, "in": true, "let": true, "module": true, "of": true,
	"port": true, "then": true, "type": true, "where": true,
}

func schemaNameFromRef(ref string) (string, error) {
	const prefix = "#/components/schemas/"
	name, ok := strings.CutPrefix(ref, prefix)
	if !ok {
		return "", fmt.Errorf("unsupported ref %q", ref)
	}
	return name, nil
}

func reserveTypeName(m *model, schemaName, source string) error {
	typeName := pascalName(schemaName)
	if previous, ok := m.UsedTypeNames[typeName]; ok && previous != source {
		return fmt.Errorf("schemas %q and %q both generate Elm type %q", previous, source, typeName)
	}
	m.UsedTypeNames[typeName] = source
	return nil
}

func uniqueInlineName(m *model, seed string) string {
	base := pascalName(seed)
	for {
		name := base
		if m.InlineCounter[base] > 0 {
			name = base + strconv.Itoa(m.InlineCounter[base]+1)
		}
		m.InlineCounter[base]++
		if _, exists := m.Schemas[name]; exists {
			continue
		}
		if _, exists := m.UsedTypeNames[pascalName(name)]; exists {
			continue
		}
		m.UsedTypeNames[pascalName(name)] = name
		return name
	}
}

func pascalName(s string) string {
	var out strings.Builder
	nextUpper := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !isASCIILetter(c) && !isASCIIDigit(c) {
			nextUpper = true
			continue
		}
		if out.Len() == 0 && isASCIIDigit(c) {
			out.WriteByte('N')
		}
		if nextUpper {
			c = toASCIIUpper(c)
			nextUpper = false
		}
		out.WriteByte(c)
	}
	if out.Len() == 0 {
		return "Generated"
	}
	return out.String()
}

func lowerCamelName(s string) string {
	p := pascalName(s)
	return string(toASCIILower(p[0])) + p[1:]
}

func safeLowerCamel(s string) string {
	name := lowerCamelName(s)
	if elmReserved[name] {
		return name + "_"
	}
	return name
}

func decoderName(schema string) string {
	return lowerCamelName(schema) + "Decoder"
}

func encoderName(schema string) string {
	return lowerCamelName(schema) + "Encoder"
}

func enumCaseName(schema string, value any) string {
	schemaPrefix := pascalName(schema)
	switch v := value.(type) {
	case string:
		name := pascalName(v)
		if name == "True" || name == "False" {
			return schemaPrefix + name
		}
		return name
	case bool:
		if v {
			return schemaPrefix + "True"
		}
		return schemaPrefix + "False"
	case float64:
		if v == float64(int64(v)) {
			return schemaPrefix + numberWord(int64(v))
		}
		return schemaPrefix + pascalName(strconv.FormatFloat(v, 'f', -1, 64))
	case int64:
		return schemaPrefix + numberWord(v)
	case int:
		return schemaPrefix + numberWord(int64(v))
	default:
		return schemaPrefix + pascalName(fmt.Sprint(v))
	}
}

func numberWord(n int64) string {
	switch n {
	case 0:
		return "Zero"
	case 1:
		return "One"
	case 2:
		return "Two"
	case 3:
		return "Three"
	case 4:
		return "Four"
	case 5:
		return "Five"
	default:
		if n < 0 {
			return "Negative" + strconv.FormatInt(-n, 10)
		}
		return strconv.FormatInt(n, 10)
	}
}

func validModuleName(s string) bool {
	if s == "" {
		return false
	}
	for part := range strings.SplitSeq(s, ".") {
		if part == "" || !isASCIIUpper(part[0]) {
			return false
		}
		for i := 0; i < len(part); i++ {
			c := part[i]
			if !(isASCIILetter(c) || isASCIIDigit(c) || c == '_') {
				return false
			}
		}
	}
	return true
}

func isASCIILetter(c byte) bool {
	return isASCIIUpper(c) || c >= 'a' && c <= 'z'
}

func isASCIIUpper(c byte) bool {
	return c >= 'A' && c <= 'Z'
}

func isASCIIDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func toASCIIUpper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - 'a' + 'A'
	}
	return c
}

func toASCIILower(c byte) byte {
	if isASCIIUpper(c) {
		return c - 'A' + 'a'
	}
	return c
}
