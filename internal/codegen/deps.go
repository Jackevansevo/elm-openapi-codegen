package codegen

import (
	"maps"
	"slices"
)

func dependencyGraph(m *model) map[string]map[string]bool {
	graph := map[string]map[string]bool{}
	for name, def := range m.Schemas {
		graph[name] = map[string]bool{}
		switch def.Kind {
		case kindRecord:
			for _, field := range def.Fields {
				for _, ref := range typeRefs(field.Type) {
					if m.Schemas[ref] != nil {
						graph[name][ref] = true
					}
				}
			}
		case kindAlias:
			for _, ref := range typeRefs(def.Type) {
				if m.Schemas[ref] != nil {
					graph[name][ref] = true
				}
			}
		case kindOneOf, kindAnyOf:
			for _, variant := range def.Variants {
				if m.Schemas[variant.Ref] != nil {
					graph[name][variant.Ref] = true
				}
			}
		}
	}
	return graph
}

func stronglyConnectedComponents(nodes []string, graph map[string]map[string]bool) [][]string {
	index := 0
	stack := []string{}
	onStack := map[string]bool{}
	indices := map[string]int{}
	lowlink := map[string]int{}
	var components [][]string

	var visit func(string)
	visit = func(v string) {
		indices[v] = index
		lowlink[v] = index
		index++
		stack = append(stack, v)
		onStack[v] = true

		neighbors := slices.Sorted(maps.Keys(graph[v]))
		for _, w := range neighbors {
			if _, ok := indices[w]; !ok {
				visit(w)
				if lowlink[w] < lowlink[v] {
					lowlink[v] = lowlink[w]
				}
			} else if onStack[w] && indices[w] < lowlink[v] {
				lowlink[v] = indices[w]
			}
		}

		if lowlink[v] == indices[v] {
			var component []string
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				component = append(component, w)
				if w == v {
					break
				}
			}
			components = append(components, component)
		}
	}

	for _, node := range nodes {
		if _, ok := indices[node]; !ok {
			visit(node)
		}
	}
	return components
}

func typeRefs(t typeExpr) []string {
	switch t.Kind {
	case typeRef:
		return []string{t.Ref}
	case typeList, typeDict:
		if t.Elem == nil {
			return nil
		}
		return typeRefs(*t.Elem)
	default:
		return nil
	}
}

func typeNeedsDict(t typeExpr) bool {
	if t.Kind == typeDict {
		return true
	}
	if t.Kind == typeList && t.Elem != nil {
		return typeNeedsDict(*t.Elem)
	}
	return false
}

func dependsOn(graph map[string]map[string]bool, from, to string) bool {
	return graph[from] != nil && graph[from][to]
}
