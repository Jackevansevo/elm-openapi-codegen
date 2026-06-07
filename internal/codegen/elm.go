package codegen

import (
	"fmt"
	"strings"
)

type elmFile struct {
	Name     string
	Exposing []string
	Imports  []string
	Decls    []elmDecl
	Trailing []elmDecl
}

type elmDecl interface {
	render(*elmWriter)
}

type elmWriter struct {
	out    strings.Builder
	indent string
}

func (w *elmWriter) line(format string, args ...any) {
	w.out.WriteString(w.indent)
	if len(args) == 0 {
		w.out.WriteString(format)
	} else {
		fmt.Fprintf(&w.out, format, args...)
	}
	w.out.WriteByte('\n')
}

func (w *elmWriter) blank() {
	w.out.WriteByte('\n')
}

func (w *elmWriter) write(s string) {
	w.out.WriteString(s)
}

func (w *elmWriter) indented(fn func()) {
	previous := w.indent
	w.indent += "    "
	fn()
	w.indent = previous
}

func (f elmFile) render() string {
	var w elmWriter
	w.line("-- %s", generatedMarker)
	w.line("module %s exposing (%s)", f.Name, strings.Join(f.Exposing, ", "))
	w.blank()
	for _, imp := range f.Imports {
		w.line("%s", imp)
	}
	w.blank()
	w.blank()

	decls := append([]elmDecl{}, f.Decls...)
	decls = append(decls, f.Trailing...)
	for i, decl := range decls {
		if i > 0 {
			w.blank()
			w.blank()
		}
		decl.render(&w)
	}
	return w.out.String()
}

type elmTypeAliasDecl struct {
	Name string
	Type string
}

func (d elmTypeAliasDecl) render(w *elmWriter) {
	w.line("type alias %s =", d.Name)
	w.indented(func() {
		w.line("%s", d.Type)
	})
}

type elmRecordAliasDecl struct {
	Name   string
	Fields []elmRecordField
}

func (d elmRecordAliasDecl) render(w *elmWriter) {
	w.line("type alias %s =", d.Name)
	w.write("    ")
	renderElmRecord(w, d.Fields, "    ")
	w.line("")
}

type elmRecursiveRecordDecl struct {
	Name   string
	Fields []elmRecordField
}

func (d elmRecursiveRecordDecl) render(w *elmWriter) {
	w.line("type %s", d.Name)
	w.line("    = %s", d.Name)
	w.write("        ")
	renderElmRecord(w, d.Fields, "        ")
	w.line("")
}

type elmRecordField struct {
	Name string
	Type string
}

func renderElmRecord(w *elmWriter, fields []elmRecordField, indent string) {
	w.write("{ ")
	for i, field := range fields {
		if i > 0 {
			w.write("\n")
			w.write(indent)
			w.write(", ")
		}
		fmt.Fprintf(&w.out, "%s : %s", field.Name, field.Type)
	}
	w.write("\n")
	w.write(indent)
	w.write("}")
}

type elmUnionDecl struct {
	Name  string
	Cases []elmUnionCase
}

type elmUnionCase struct {
	Name string
	Args []string
}

func (d elmUnionDecl) render(w *elmWriter) {
	w.line("type %s", d.Name)
	for i, c := range d.Cases {
		prefix := "    = "
		if i > 0 {
			prefix = "    | "
		}
		if len(c.Args) == 0 {
			w.line("%s%s", prefix, c.Name)
			continue
		}
		w.line("%s%s %s", prefix, c.Name, strings.Join(c.Args, " "))
	}
}

type elmFunctionDecl struct {
	Name      string
	Args      []string
	Signature string
	Body      elmExpr
}

func (d elmFunctionDecl) render(w *elmWriter) {
	w.line("%s : %s", d.Name, d.Signature)
	name := d.Name
	if len(d.Args) > 0 {
		name += " " + strings.Join(d.Args, " ")
	}
	w.line("%s =", name)
	d.Body.render(w, "    ")
}

type elmExpr interface {
	String() string
	render(*elmWriter, string)
}

func elmExprString(expr elmExpr) string {
	var w elmWriter
	expr.render(&w, "")
	return strings.TrimSuffix(w.out.String(), "\n")
}

type elmVarExpr struct {
	Name string
}

func (e elmVarExpr) render(w *elmWriter, indent string) {
	w.write(indent)
	w.write(e.Name)
	w.write("\n")
}

func (e elmVarExpr) String() string {
	return elmExprString(e)
}

func (e elmVarExpr) renderInline() string {
	return e.Name
}

type elmIntExpr struct {
	Value string
}

func (e elmIntExpr) render(w *elmWriter, indent string) {
	w.write(indent)
	w.write(e.Value)
	w.write("\n")
}

func (e elmIntExpr) String() string {
	return elmExprString(e)
}

func (e elmIntExpr) renderInline() string {
	return e.Value
}

type elmStringExpr struct {
	Value string
}

func (e elmStringExpr) render(w *elmWriter, indent string) {
	w.write(indent)
	w.write(elmStringLiteral(e.Value))
	w.write("\n")
}

func (e elmStringExpr) String() string {
	return elmExprString(e)
}

func (e elmStringExpr) renderInline() string {
	return elmStringLiteral(e.Value)
}

type elmLiteralExpr struct {
	Value string
}

func (e elmLiteralExpr) render(w *elmWriter, indent string) {
	w.write(indent)
	w.write(e.Value)
	w.write("\n")
}

func (e elmLiteralExpr) String() string {
	return elmExprString(e)
}

func (e elmLiteralExpr) renderInline() string {
	return e.Value
}

type elmCallExpr struct {
	Fn   elmExpr
	Args []elmExpr
}

func (e elmCallExpr) render(w *elmWriter, indent string) {
	if inline, ok := elmRenderInline(e); ok {
		w.write(indent)
		w.write(inline)
		w.write("\n")
		return
	}

	w.write(indent)
	w.write(elmInlineOrRendered(e.Fn))
	w.write("\n")
	for _, arg := range e.Args {
		arg.render(w, indent+"    ")
	}
}

func (e elmCallExpr) String() string {
	return elmExprString(e)
}

type elmBinOpExpr struct {
	Left  elmExpr
	Op    string
	Right elmExpr
}

func (e elmBinOpExpr) render(w *elmWriter, indent string) {
	w.write(indent)
	w.write(e.renderInline())
	w.write("\n")
}

func (e elmBinOpExpr) String() string {
	return elmExprString(e)
}

func (e elmBinOpExpr) renderInline() string {
	return elmInlineOrRendered(e.Left) + " " + e.Op + " " + elmInlineOrRendered(e.Right)
}

type elmIfExpr struct {
	Condition elmExpr
	Then      elmExpr
	Else      elmExpr
}

func (e elmIfExpr) render(w *elmWriter, indent string) {
	w.write(indent)
	w.write("if ")
	w.write(elmInlineOrRendered(e.Condition))
	w.write(" then\n")
	e.Then.render(w, indent+"    ")
	w.write("\n")
	w.write(indent)
	w.write("else\n")
	e.Else.render(w, indent+"    ")
}

func (e elmIfExpr) String() string {
	return elmExprString(e)
}

type elmPipeExpr struct {
	Start elmExpr
	Steps []elmExpr
}

func (e elmPipeExpr) render(w *elmWriter, indent string) {
	e.Start.render(w, indent)
	for _, step := range e.Steps {
		w.write(indent)
		w.write("    |> ")
		if inline, ok := elmRenderInline(step); ok {
			w.write(inline)
			w.write("\n")
			continue
		}
		if call, ok := step.(elmCallExpr); ok && len(call.Args) > 0 {
			w.write(elmInlineOrRendered(call.Fn))
			w.write("\n")
			for _, arg := range call.Args {
				arg.render(w, indent+"        ")
			}
			continue
		}
		w.write(elmInlineOrRendered(step))
		w.write("\n")
	}
}

func (e elmPipeExpr) String() string {
	return elmExprString(e)
}

type elmLambdaExpr struct {
	Args []elmPattern
	Body elmExpr
}

func (e elmLambdaExpr) render(w *elmWriter, indent string) {
	w.write(indent)
	w.write("(")
	w.write("\\")
	for i, arg := range e.Args {
		if i > 0 {
			w.write(" ")
		}
		w.write(arg.renderPattern())
	}
	w.write(" ->\n")
	e.Body.render(w, indent+"    ")
	w.write(indent)
	w.write(")\n")
}

func (e elmLambdaExpr) String() string {
	return elmExprString(e)
}

type elmCaseExpr struct {
	Expr     elmExpr
	Branches []elmCaseBranch
}

type elmCaseBranch struct {
	Pattern elmPattern
	Body    elmExpr
}

func (e elmCaseExpr) render(w *elmWriter, indent string) {
	w.write(indent)
	w.write("case ")
	w.write(elmInlineOrRendered(e.Expr))
	w.write(" of\n")
	elmRenderCaseBranches(w, indent, e.Branches)
}

func (e elmCaseExpr) String() string {
	return elmExprString(e)
}

type elmListExpr struct {
	Items []elmExpr
}

func (e elmListExpr) render(w *elmWriter, indent string) {
	if len(e.Items) == 0 {
		w.write(indent)
		w.write("[]\n")
		return
	}
	for i, item := range e.Items {
		w.write(indent)
		if i == 0 {
			w.write("[ ")
		} else {
			w.write(", ")
		}
		w.write(elmInlineOrRendered(item))
		w.write("\n")
	}
	w.write(indent)
	w.write("]\n")
}

func (e elmListExpr) String() string {
	return elmExprString(e)
}

type elmRecordExpr struct {
	Fields []elmRecordExprField
}

type elmRecordExprField struct {
	Name  string
	Value elmExpr
}

func (e elmRecordExpr) render(w *elmWriter, indent string) {
	if len(e.Fields) == 0 {
		w.write(indent)
		w.write("{}\n")
		return
	}
	for i, field := range e.Fields {
		w.write(indent)
		if i == 0 {
			w.write("{ ")
		} else {
			w.write(", ")
		}
		w.write(field.Name)
		w.write(" = ")
		w.write(elmInlineOrRendered(field.Value))
		w.write("\n")
	}
	w.write(indent)
	w.write("}\n")
}

func (e elmRecordExpr) String() string {
	return elmExprString(e)
}

type elmParensExpr struct {
	Expr elmExpr
}

func (e elmParensExpr) render(w *elmWriter, indent string) {
	if c, ok := e.Expr.(elmCaseExpr); ok {
		w.write(indent)
		w.write("(case ")
		w.write(elmInlineOrRendered(c.Expr))
		w.write(" of\n")
		elmRenderCaseBranches(w, indent, c.Branches)
		w.write(indent)
		w.write(")\n")
		return
	}
	if inline, ok := elmRenderInline(e.Expr); ok {
		w.write(indent)
		w.write("(")
		w.write(inline)
		w.write(")\n")
		return
	}
	w.write(indent)
	w.write("(")
	renderMultilineExprAfterOpenParen(w, e.Expr, indent)
	w.write(indent)
	w.write(")\n")
}

func (e elmParensExpr) String() string {
	return elmExprString(e)
}

type elmInlineExpr interface {
	renderInline() string
}

func elmRenderInline(expr elmExpr) (string, bool) {
	switch e := expr.(type) {
	case elmParensExpr:
		inline, ok := elmRenderInline(e.Expr)
		if !ok {
			return "", false
		}
		return "(" + inline + ")", true
	case elmInlineExpr:
		return e.renderInline(), true
	case elmCallExpr:
		fn, ok := elmRenderInline(e.Fn)
		if !ok {
			return "", false
		}
		parts := []string{fn}
		for _, arg := range e.Args {
			inline, ok := elmRenderInlineArg(arg)
			if !ok {
				return "", false
			}
			parts = append(parts, inline)
		}
		return strings.Join(parts, " "), true
	}
	return "", false
}

func elmRenderInlineArg(expr elmExpr) (string, bool) {
	inline, ok := elmRenderInline(expr)
	if !ok {
		return "", false
	}
	if _, ok := expr.(elmCallExpr); ok {
		return "(" + inline + ")", true
	}
	return inline, true
}

func elmInlineOrRendered(expr elmExpr) string {
	if inline, ok := elmRenderInline(expr); ok {
		return inline
	}
	var w elmWriter
	expr.render(&w, "")
	return strings.TrimSpace(w.out.String())
}

func renderMultilineExprAfterOpenParen(w *elmWriter, expr elmExpr, indent string) {
	switch e := expr.(type) {
	case elmPipeExpr:
		renderExprAfterPrefix(w, e.Start, indent)
		for _, step := range e.Steps {
			w.write(indent)
			w.write("    |> ")
			if inline, ok := elmRenderInline(step); ok {
				w.write(inline)
				w.write("\n")
				continue
			}
			if call, ok := step.(elmCallExpr); ok && len(call.Args) > 0 {
				w.write(elmInlineOrRendered(call.Fn))
				w.write("\n")
				for _, arg := range call.Args {
					arg.render(w, indent+"        ")
				}
				continue
			}
			w.write(elmInlineOrRendered(step))
			w.write("\n")
		}
	default:
		w.write("\n")
		expr.render(w, indent+"    ")
	}
}

func renderExprAfterPrefix(w *elmWriter, expr elmExpr, indent string) {
	if inline, ok := elmRenderInline(expr); ok {
		w.write(inline)
		w.write("\n")
		return
	}
	if call, ok := expr.(elmCallExpr); ok {
		w.write(elmInlineOrRendered(call.Fn))
		w.write("\n")
		for _, arg := range call.Args {
			arg.render(w, indent+"    ")
		}
		return
	}
	w.write("\n")
	expr.render(w, indent+"    ")
}

func elmRenderCaseBranches(w *elmWriter, caseIndent string, branches []elmCaseBranch) {
	for i, branch := range branches {
		if i > 0 {
			w.write("\n")
		}
		w.write(caseIndent)
		w.write("    ")
		w.write(branch.Pattern.renderPattern())
		w.write(" ->\n")
		branch.Body.render(w, caseIndent+"        ")
	}
}

type elmPattern interface {
	renderPattern() string
}

type elmVarPattern struct {
	Name string
}

func (p elmVarPattern) renderPattern() string {
	return p.Name
}

type elmIntPattern struct {
	Value string
}

func (p elmIntPattern) renderPattern() string {
	return p.Value
}

type elmStringPattern struct {
	Value string
}

func (p elmStringPattern) renderPattern() string {
	return elmStringLiteral(p.Value)
}
