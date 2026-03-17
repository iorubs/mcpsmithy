package schema

import (
	_ "embed"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"strconv"
	"strings"
)

// ParamTypeSource is the raw Go source of paramtype.go, embedded at compile time.
// Callers include this alongside version-specific type sources when calling
// ParseTypeDocs so that ParamType docs are available.
//
//go:embed paramtype.go
var ParamTypeSource string

// ParseTypeDocs parses Go source files and extracts doc comments for types,
// struct fields, and typed string const values into a DocProvider.
// Accepts multiple source strings; results are merged with later sources
// overwriting earlier ones on collision.
func ParseTypeDocs(srcs ...string) DocProvider {
	types := map[string]string{}
	fields := map[string]map[string]string{}
	values := map[string]map[string]string{}

	for _, src := range srcs {
		p := parseSingleSource(src)
		maps.Copy(types, p.Types)
		maps.Copy(fields, p.Fields)
		for k, v := range p.Values {
			if values[k] == nil {
				values[k] = map[string]string{}
			}
			maps.Copy(values[k], v)
		}
	}

	return DocProvider{Types: types, Fields: fields, Values: values}
}

// parseSingleSource parses one Go source string into a DocProvider.
func parseSingleSource(src string) DocProvider {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "types.go", src, parser.ParseComments)
	if err != nil {
		panic("schema: failed to parse types source: " + err.Error())
	}

	types := map[string]string{}
	fields := map[string]map[string]string{}
	values := map[string]map[string]string{}

	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		switch gd.Tok {
		case token.TYPE:
			for _, spec := range gd.Specs {
				ts := spec.(*ast.TypeSpec)
				name := ts.Name.Name

				doc := commentText(ts.Doc)
				if doc == "" {
					doc = commentText(gd.Doc)
				}
				if doc != "" {
					types[name] = doc
				}

				st, ok := ts.Type.(*ast.StructType)
				if !ok || st.Fields == nil {
					continue
				}
				fm := map[string]string{}
				for _, f := range st.Fields.List {
					if len(f.Names) == 0 || f.Doc == nil {
						continue
					}
					fm[f.Names[0].Name] = commentText(f.Doc)
				}
				if len(fm) > 0 {
					fields[name] = fm
				}
			}

		case token.CONST:
			for _, spec := range gd.Specs {
				vs := spec.(*ast.ValueSpec)
				if vs.Type == nil || len(vs.Names) == 0 || len(vs.Values) == 0 {
					continue
				}
				typeName := typeNameFromExpr(vs.Type)
				if typeName == "" {
					continue
				}
				val := constStringValue(vs.Values[0])
				if val == "" {
					continue
				}
				doc := commentText(vs.Doc)
				if doc == "" {
					continue
				}
				if values[typeName] == nil {
					values[typeName] = map[string]string{}
				}
				values[typeName][val] = doc
			}
		}
	}

	return DocProvider{Types: types, Fields: fields, Values: values}
}

func commentText(cg *ast.CommentGroup) string {
	if cg == nil {
		return ""
	}
	var lines []string
	for _, c := range cg.List {
		text := c.Text
		switch {
		case strings.HasPrefix(text, "// "):
			text = text[3:]
		case strings.HasPrefix(text, "//"):
			text = text[2:]
		default:
			continue
		}
		lines = append(lines, text)
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n ")
}

func typeNameFromExpr(expr ast.Expr) string {
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func constStringValue(expr ast.Expr) string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return ""
	}
	return s
}
