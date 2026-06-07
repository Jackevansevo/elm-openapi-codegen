package codegen

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func isSupportedEnumValue(value any) bool {
	switch value.(type) {
	case string, bool, float64, int64, int:
		return true
	default:
		return false
	}
}

func elmDefault(t typeExpr, value any) (string, error) {
	switch t.Kind {
	case typeString:
		v, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("default %v is not a string", value)
		}
		return elmStringLiteral(v), nil
	case typeBool:
		v, ok := value.(bool)
		if !ok {
			return "", fmt.Errorf("default %v is not a bool", value)
		}
		if v {
			return "True", nil
		}
		return "False", nil
	case typeInt:
		v, ok := intDefault(value)
		if !ok {
			return "", fmt.Errorf("default %v is not an int", value)
		}
		return strconv.FormatInt(v, 10), nil
	case typeFloat:
		v, ok := floatDefault(value)
		if !ok {
			return "", fmt.Errorf("default %v is not a number", value)
		}
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	default:
		return "", fmt.Errorf("defaults for %s fields are unsupported", elmType(t))
	}
}

func intDefault(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case float64:
		if v == float64(int64(v)) {
			return int64(v), true
		}
	}
	return 0, false
}

func floatDefault(value any) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	}
	return 0, false
}

func elmStringLiteral(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func wrapType(t string) string {
	if strings.Contains(t, " ") {
		return "(" + t + ")"
	}
	return t
}
