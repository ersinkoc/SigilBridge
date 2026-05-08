package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"unicode"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	src := filepath.Join("internal", "admin", "types.go")
	dst := filepath.Join("ui", "src", "types", "api.ts")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, src, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	buf.WriteString("export type ApiError = {\n  error?: string | { message?: string };\n};\n\n")
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || !ts.Name.IsExported() {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok || !strings.HasSuffix(ts.Name.Name, "DTO") && !strings.HasSuffix(ts.Name.Name, "Request") && !strings.HasSuffix(ts.Name.Name, "Response") && ts.Name.Name != "ReloadResult" {
				continue
			}
			writeStruct(&buf, ts.Name.Name, st)
		}
	}
	output := strings.TrimRight(buf.String(), "\n") + "\n"
	// #nosec G306 -- generated TypeScript source is not secret material and should be readable in the repo.
	return os.WriteFile(dst, []byte(output), 0o644)
}

func writeStruct(buf *bytes.Buffer, name string, st *ast.StructType) {
	var embeds []string
	var fields []string
	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			if ident, ok := field.Type.(*ast.Ident); ok {
				embeds = append(embeds, ident.Name)
			}
			continue
		}
		jsonName, optional := jsonField(field)
		if jsonName == "-" {
			continue
		}
		if jsonName == "" {
			jsonName = lowerFirst(field.Names[0].Name)
		}
		suffix := ""
		if optional {
			suffix = "?"
		}
		fields = append(fields, fmt.Sprintf("  %s%s: %s;", jsonName, suffix, tsType(field.Type)))
	}
	if len(embeds) > 0 {
		fmt.Fprintf(buf, "export type %s = %s & {\n", name, strings.Join(embeds, " & "))
	} else {
		fmt.Fprintf(buf, "export type %s = {\n", name)
	}
	for _, field := range fields {
		buf.WriteString(field)
		buf.WriteByte('\n')
	}
	buf.WriteString("};\n\n")
}

func jsonField(field *ast.Field) (string, bool) {
	if field.Tag == nil {
		return "", false
	}
	tag := strings.Trim(field.Tag.Value, "`")
	value := reflect.StructTag(tag).Get("json")
	if value == "" {
		return "", false
	}
	parts := strings.Split(value, ",")
	return parts[0], strings.Contains(value, "omitempty")
}

func tsType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		switch t.Name {
		case "string":
			return "string"
		case "bool":
			return "boolean"
		case "int", "int64", "float64":
			return "number"
		case "any":
			return "unknown"
		default:
			return t.Name
		}
	case *ast.ArrayType:
		return "Array<" + tsType(t.Elt) + ">"
	case *ast.MapType:
		return "Record<string, " + tsType(t.Value) + ">"
	default:
		return "unknown"
	}
}

func lowerFirst(value string) string {
	if value == "" {
		return value
	}
	runes := []rune(value)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}
