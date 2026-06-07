package codegen

import (
	"fmt"
	"strconv"
)

func schemaToElmDecls(def *schemaDef) ([]elmDecl, error) {
	typeName := pascalName(def.Name)
	switch def.Kind {
	case kindAlias:
		if def.Recursive {
			return []elmDecl{
				elmUnionDecl{
					Name: typeName,
					Cases: []elmUnionCase{
						{Name: typeName, Args: []string{wrapType(elmType(def.Type))}},
					},
				},
				elmFunctionDecl{
					Name:      decoderName(def.Name),
					Signature: "Decoder " + typeName,
					Body: elmCallExpr{
						Fn: elmVarExpr{Name: "Decode.lazy"},
						Args: []elmExpr{
							elmLambdaExpr{
								Args: []elmPattern{elmVarPattern{Name: "_"}},
								Body: elmCallExpr{
									Fn: elmVarExpr{Name: "Decode.map"},
									Args: []elmExpr{
										elmVarExpr{Name: typeName},
										decoderExpr(def.Type),
									},
								},
							},
						},
					},
				},
			}, nil
		}
		return []elmDecl{
			elmTypeAliasDecl{Name: typeName, Type: elmType(def.Type)},
			elmFunctionDecl{
				Name:      decoderName(def.Name),
				Signature: "Decoder " + typeName,
				Body:      decoderExpr(def.Type),
			},
		}, nil
	case kindRecord:
		recordDecl := elmDecl(elmRecordAliasDecl{Name: typeName, Fields: elmRecordFields(def.Fields)})
		if def.Recursive {
			recordDecl = elmRecursiveRecordDecl{Name: typeName, Fields: elmRecordFields(def.Fields)}
		}
		return []elmDecl{
			recordDecl,
			elmFunctionDecl{
				Name:      decoderName(def.Name),
				Signature: "Decoder " + typeName,
				Body:      recordDecoderBody(def),
			},
		}, nil
	case kindEnum:
		return []elmDecl{
			elmUnionDecl{Name: typeName, Cases: enumUnionCases(def)},
			elmFunctionDecl{
				Name:      decoderName(def.Name),
				Signature: "Decoder " + typeName,
				Body:      enumDecoderBody(def),
			},
			elmFunctionDecl{
				Name:      encoderName(def.Name),
				Args:      []string{"value"},
				Signature: typeName + " -> Encode.Value",
				Body:      enumEncoderBody(def),
			},
		}, nil
	case kindOneOf:
		return []elmDecl{
			elmUnionDecl{Name: typeName, Cases: variantUnionCases(def)},
			elmFunctionDecl{
				Name:      decoderName(def.Name),
				Signature: "Decoder " + typeName,
				Body:      oneOfDecoderBody(def),
			},
		}, nil
	case kindAnyOf:
		return []elmDecl{
			elmUnionDecl{Name: typeName, Cases: variantUnionCases(def)},
			elmFunctionDecl{
				Name:      decoderName(def.Name),
				Signature: "Decoder " + typeName,
				Body:      anyOfDecoderBody(def),
			},
		}, nil
	default:
		return nil, fmt.Errorf("%s: unsupported schema kind", def.Name)
	}
}

func elmRecordFields(fields []fieldDef) []elmRecordField {
	var out []elmRecordField
	for _, field := range fields {
		out = append(out, elmRecordField{Name: field.ElmName, Type: fieldElmType(field)})
	}
	return out
}

func enumUnionCases(def *schemaDef) []elmUnionCase {
	var cases []elmUnionCase
	for _, c := range def.Enum {
		cases = append(cases, elmUnionCase{Name: c.Name})
	}
	return cases
}

func variantUnionCases(def *schemaDef) []elmUnionCase {
	var cases []elmUnionCase
	for _, variant := range def.Variants {
		cases = append(cases, elmUnionCase{
			Name: variant.Name,
			Args: []string{pascalName(variant.Ref)},
		})
	}
	return cases
}

func recordDecoderBody(def *schemaDef) elmExpr {
	object := recordDecoderObject(def)
	if !def.Recursive {
		return object
	}
	return elmCallExpr{
		Fn: elmVarExpr{Name: "Decode.lazy"},
		Args: []elmExpr{
			elmLambdaExpr{
				Args: []elmPattern{elmVarPattern{Name: "_"}},
				Body: object,
			},
		},
	}
}

func enumDecoderBody(def *schemaDef) elmExpr {
	var items []elmExpr
	for _, c := range def.Enum {
		items = append(items, elmCallExpr{
			Fn: elmVarExpr{Name: "Decode.map"},
			Args: []elmExpr{
				elmCallExpr{
					Fn:   elmVarExpr{Name: "always"},
					Args: []elmExpr{elmVarExpr{Name: c.Name}},
				},
				enumValueDecoder(c.Value),
			},
		})
	}
	return elmCallExpr{
		Fn: elmVarExpr{Name: "Decode.oneOf"},
		Args: []elmExpr{
			elmListExpr{Items: items},
		},
	}
}

func enumEncoderBody(def *schemaDef) elmExpr {
	var branches []elmCaseBranch
	for _, c := range def.Enum {
		branches = append(branches, elmCaseBranch{
			Pattern: elmVarPattern{Name: c.Name},
			Body:    enumValueEncoder(c.Value),
		})
	}
	return elmCaseExpr{
		Expr:     elmVarExpr{Name: "value"},
		Branches: branches,
	}
}

func enumValueDecoder(value any) elmExpr {
	fn := enumExactFunctionName(value)
	return elmCallExpr{
		Fn:   elmVarExpr{Name: fn},
		Args: []elmExpr{enumValueLiteral(value)},
	}
}

func enumValueEncoder(value any) elmExpr {
	switch v := value.(type) {
	case string:
		return elmCallExpr{Fn: elmVarExpr{Name: "Encode.string"}, Args: []elmExpr{elmStringExpr{Value: v}}}
	case bool:
		literal := "False"
		if v {
			literal = "True"
		}
		return elmCallExpr{Fn: elmVarExpr{Name: "Encode.bool"}, Args: []elmExpr{elmLiteralExpr{Value: literal}}}
	case float64:
		if v == float64(int64(v)) {
			return elmCallExpr{Fn: elmVarExpr{Name: "Encode.int"}, Args: []elmExpr{elmIntExpr{Value: strconv.FormatInt(int64(v), 10)}}}
		}
		return elmCallExpr{Fn: elmVarExpr{Name: "Encode.float"}, Args: []elmExpr{elmLiteralExpr{Value: strconv.FormatFloat(v, 'f', -1, 64)}}}
	case int64:
		return elmCallExpr{Fn: elmVarExpr{Name: "Encode.int"}, Args: []elmExpr{elmIntExpr{Value: strconv.FormatInt(v, 10)}}}
	case int:
		return elmCallExpr{Fn: elmVarExpr{Name: "Encode.int"}, Args: []elmExpr{elmIntExpr{Value: strconv.Itoa(v)}}}
	default:
		return elmCallExpr{Fn: elmVarExpr{Name: "Encode.string"}, Args: []elmExpr{elmStringExpr{Value: fmt.Sprint(value)}}}
	}
}

func enumValueLiteral(value any) elmExpr {
	switch v := value.(type) {
	case string:
		return elmStringExpr{Value: v}
	case bool:
		if v {
			return elmLiteralExpr{Value: "True"}
		}
		return elmLiteralExpr{Value: "False"}
	case float64:
		if v == float64(int64(v)) {
			return elmIntExpr{Value: strconv.FormatInt(int64(v), 10)}
		}
		return elmLiteralExpr{Value: strconv.FormatFloat(v, 'f', -1, 64)}
	case int64:
		return elmIntExpr{Value: strconv.FormatInt(v, 10)}
	case int:
		return elmIntExpr{Value: strconv.Itoa(v)}
	default:
		return elmStringExpr{Value: fmt.Sprint(value)}
	}
}

func enumExactFunctionName(value any) string {
	switch v := value.(type) {
	case string:
		return "exactString"
	case bool:
		return "exactBool"
	case float64:
		if v == float64(int64(v)) {
			return "exactInt"
		}
		return "exactFloat"
	case int, int64:
		return "exactInt"
	default:
		return "exactString"
	}
}

func oneOfDecoderBody(def *schemaDef) elmExpr {
	if def.UnionDiscriminator == "" {
		return anyOfDecoderBody(def)
	}

	typeName := pascalName(def.Name)
	discriminator := def.UnionDiscriminator
	var branches []elmCaseBranch
	for _, variant := range def.Variants {
		discriminatorValue := variant.Discriminator
		if discriminatorValue == "" {
			discriminatorValue = lowerCamelName(variant.Ref)
		}
		branches = append(branches, elmCaseBranch{
			Pattern: elmStringPattern{Value: discriminatorValue},
			Body: elmCallExpr{
				Fn: elmVarExpr{Name: "Decode.map"},
				Args: []elmExpr{
					elmVarExpr{Name: variant.Name},
					elmVarExpr{Name: decoderName(variant.Ref)},
				},
			},
		})
	}
	branches = append(branches, elmCaseBranch{
		Pattern: elmVarPattern{Name: "other"},
		Body:    unknownDecoderFail(typeName, elmVarExpr{Name: "other"}),
	})
	return elmPipeExpr{
		Start: elmCallExpr{
			Fn: elmVarExpr{Name: "Decode.field"},
			Args: []elmExpr{
				elmStringExpr{Value: discriminator},
				elmVarExpr{Name: "Decode.string"},
			},
		},
		Steps: []elmExpr{
			elmCallExpr{
				Fn: elmVarExpr{Name: "Decode.andThen"},
				Args: []elmExpr{
					elmLambdaExpr{
						Args: []elmPattern{elmVarPattern{Name: "value"}},
						Body: elmCaseExpr{
							Expr:     elmVarExpr{Name: "value"},
							Branches: branches,
						},
					},
				},
			},
		},
	}
}

func unknownDecoderFail(typeName string, other elmExpr) elmExpr {
	return elmCallExpr{
		Fn: elmVarExpr{Name: "Decode.fail"},
		Args: []elmExpr{
			elmParensExpr{
				Expr: elmBinOpExpr{
					Left:  elmStringExpr{Value: "Unknown " + typeName + ": "},
					Op:    "++",
					Right: other,
				},
			},
		},
	}
}

func anyOfDecoderBody(def *schemaDef) elmExpr {
	var items []elmExpr
	for _, variant := range def.Variants {
		items = append(items, elmCallExpr{
			Fn: elmVarExpr{Name: "Decode.map"},
			Args: []elmExpr{
				elmVarExpr{Name: variant.Name},
				elmVarExpr{Name: decoderName(variant.Ref)},
			},
		})
	}
	return elmCallExpr{
		Fn: elmVarExpr{Name: "Decode.oneOf"},
		Args: []elmExpr{
			elmListExpr{Items: items},
		},
	}
}

func recordDecoderObject(def *schemaDef) elmExpr {
	return recordDecoderPipeline(def)
}

func recordDecoderPipeline(def *schemaDef) elmExpr {
	typeName := pascalName(def.Name)
	var start elmExpr
	if def.Recursive {
		start = elmCallExpr{
			Fn: elmVarExpr{Name: "Decode.succeed"},
			Args: []elmExpr{
				elmLambdaExpr{
					Args: recordDecoderArgs(def.Fields),
					Body: elmCallExpr{
						Fn: elmVarExpr{Name: typeName},
						Args: []elmExpr{
							recordConstructorFields(def.Fields),
						},
					},
				},
			},
		}
	} else {
		start = elmCallExpr{
			Fn:   elmVarExpr{Name: "Decode.succeed"},
			Args: []elmExpr{elmVarExpr{Name: typeName}},
		}
	}
	var steps []elmExpr
	for _, field := range def.Fields {
		steps = append(steps, pipelineStep(field))
	}
	return elmPipeExpr{Start: start, Steps: steps}
}

func recordDecoderArgs(fields []fieldDef) []elmPattern {
	var args []elmPattern
	for _, field := range fields {
		args = append(args, elmVarPattern{Name: field.ElmName})
	}
	return args
}

func recordConstructorFields(fields []fieldDef) elmExpr {
	var exprFields []elmRecordExprField
	for _, field := range fields {
		exprFields = append(exprFields, elmRecordExprField{
			Name:  field.ElmName,
			Value: elmVarExpr{Name: field.ElmName},
		})
	}
	return elmRecordExpr{Fields: exprFields}
}

func pipelineStep(field fieldDef) elmExpr {
	fieldDecoder := decoderExpr(field.Type)
	if field.Exact != nil {
		fieldDecoder = elmParensExpr{
			Expr: elmCallExpr{
				Fn:   elmVarExpr{Name: "exactString"},
				Args: []elmExpr{elmStringExpr{Value: *field.Exact}},
			},
		}
	}
	if field.Nullable {
		fieldDecoder = elmCallExpr{
			Fn:   elmVarExpr{Name: "Decode.nullable"},
			Args: []elmExpr{fieldDecoder},
		}
	}
	if field.Required {
		return elmCallExpr{
			Fn: elmVarExpr{Name: "required"},
			Args: []elmExpr{
				elmStringExpr{Value: field.JSONName},
				fieldDecoder,
			},
		}
	}
	if field.Default != nil {
		return elmCallExpr{
			Fn: elmVarExpr{Name: "optional"},
			Args: []elmExpr{
				elmStringExpr{Value: field.JSONName},
				fieldDecoder,
				elmLiteralExpr{Value: *field.Default},
			},
		}
	}
	if field.Nullable {
		return elmCallExpr{
			Fn: elmVarExpr{Name: "optional"},
			Args: []elmExpr{
				elmStringExpr{Value: field.JSONName},
				fieldDecoder,
				elmVarExpr{Name: "Nothing"},
			},
		}
	}
	return elmCallExpr{
		Fn: elmVarExpr{Name: "optional"},
		Args: []elmExpr{
			elmStringExpr{Value: field.JSONName},
			elmParensExpr{
				Expr: elmCallExpr{
					Fn: elmVarExpr{Name: "Decode.map"},
					Args: []elmExpr{
						elmVarExpr{Name: "Just"},
						fieldDecoder,
					},
				},
			},
			elmVarExpr{Name: "Nothing"},
		},
	}
}

func decoderExpr(t typeExpr) elmExpr {
	switch t.Kind {
	case typeString:
		return elmVarExpr{Name: "Decode.string"}
	case typeInt:
		return elmVarExpr{Name: "Decode.int"}
	case typeFloat:
		return elmVarExpr{Name: "Decode.float"}
	case typeBool:
		return elmVarExpr{Name: "Decode.bool"}
	case typeValue, typeUnknown:
		return elmVarExpr{Name: "Decode.value"}
	case typeRef:
		return elmVarExpr{Name: decoderName(t.Ref)}
	case typeList:
		return elmCallExpr{
			Fn:   elmVarExpr{Name: "Decode.list"},
			Args: []elmExpr{decoderExpr(*t.Elem)},
		}
	case typeDict:
		return elmCallExpr{
			Fn:   elmVarExpr{Name: "Decode.dict"},
			Args: []elmExpr{decoderExpr(*t.Elem)},
		}
	default:
		return elmVarExpr{Name: "Decode.value"}
	}
}

func fieldElmType(field fieldDef) string {
	t := elmType(field.Type)
	if field.Nullable {
		return "Maybe " + wrapType(t)
	}
	if field.Required || field.Default != nil {
		return t
	}
	return "Maybe " + wrapType(t)
}

func elmType(t typeExpr) string {
	switch t.Kind {
	case typeString:
		return "String"
	case typeInt:
		return "Int"
	case typeFloat:
		return "Float"
	case typeBool:
		return "Bool"
	case typeValue, typeUnknown:
		return "Decode.Value"
	case typeRef:
		return pascalName(t.Ref)
	case typeList:
		return "List " + wrapType(elmType(*t.Elem))
	case typeDict:
		return "Dict String " + wrapType(elmType(*t.Elem))
	default:
		return "Decode.Value"
	}
}

func exactFunctionNamesForModule(m *model, names []string) []string {
	needed := map[string]bool{}
	for _, name := range names {
		def := m.Schemas[name]
		switch def.Kind {
		case kindRecord:
			for _, field := range def.Fields {
				if field.Exact != nil {
					needed["exactString"] = true
				}
			}
		case kindEnum:
			for _, c := range def.Enum {
				needed[enumExactFunctionName(c.Value)] = true
			}
		}
	}
	order := []string{"exactString", "exactInt", "exactFloat", "exactBool"}
	var out []string
	for _, name := range order {
		if needed[name] {
			out = append(out, name)
		}
	}
	return out
}

func exactValueDecl(name string) elmDecl {
	typ := "String"
	decoder := "Decode.string"
	switch name {
	case "exactInt":
		typ = "Int"
		decoder = "Decode.int"
	case "exactFloat":
		typ = "Float"
		decoder = "Decode.float"
	case "exactBool":
		typ = "Bool"
		decoder = "Decode.bool"
	}
	return elmFunctionDecl{
		Name:      name,
		Args:      []string{"expected"},
		Signature: typ + " -> Decoder " + typ,
		Body: elmPipeExpr{
			Start: elmVarExpr{Name: decoder},
			Steps: []elmExpr{
				elmCallExpr{
					Fn: elmVarExpr{Name: "Decode.andThen"},
					Args: []elmExpr{
						elmLambdaExpr{
							Args: []elmPattern{elmVarPattern{Name: "actual"}},
							Body: elmIfExpr{
								Condition: elmBinOpExpr{
									Left:  elmVarExpr{Name: "actual"},
									Op:    "==",
									Right: elmVarExpr{Name: "expected"},
								},
								Then: elmCallExpr{
									Fn:   elmVarExpr{Name: "Decode.succeed"},
									Args: []elmExpr{elmVarExpr{Name: "actual"}},
								},
								Else: elmCallExpr{
									Fn: elmVarExpr{Name: "Decode.fail"},
									Args: []elmExpr{
										elmStringExpr{Value: "Unexpected value"},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
