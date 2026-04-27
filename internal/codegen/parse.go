package codegen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
)

type funcSpec struct {
	CName  string
	GoName string
	Return cType
	Params []paramSpec
}

type paramSpec struct {
	Name string
	Type cType
}

type cType struct {
	GoName string
	Size   int
	Align  int
	Move   string
	Load   string
	Kind   cKind
	Unsafe bool
}

type cKind uint8

const (
	scalarType cKind = iota
	voidType
	bytesType
	stringType
	bytesPtrType
)

var funcDeclRE = regexp.MustCompile(`(?m)^\s*([A-Za-z_][A-Za-z0-9_]*(?:\s+[A-Za-z_][A-Za-z0-9_]*)*?\s*\*?)\s*([A-Za-z_][A-Za-z0-9_]*)\s*\(([^)]*)\)\s*\{`)

var (
	i8    = cType{GoName: "int8", Size: 1, Align: 1, Move: "MOVB", Load: "MOVBLSX"}
	u8    = cType{GoName: "uint8", Size: 1, Align: 1, Move: "MOVB", Load: "MOVBLZX"}
	i16   = cType{GoName: "int16", Size: 2, Align: 2, Move: "MOVW", Load: "MOVWLSX"}
	u16   = cType{GoName: "uint16", Size: 2, Align: 2, Move: "MOVW", Load: "MOVWLZX"}
	i32   = cType{GoName: "int32", Size: 4, Align: 4, Move: "MOVL", Load: "MOVL"}
	u32   = cType{GoName: "uint32", Size: 4, Align: 4, Move: "MOVL", Load: "MOVL"}
	i64   = cType{GoName: "int64", Size: 8, Align: 8, Move: "MOVQ", Load: "MOVQ"}
	u64   = cType{GoName: "uint64", Size: 8, Align: 8, Move: "MOVQ", Load: "MOVQ"}
	bytes = cType{GoName: "[]byte", Size: 24, Align: 8, Kind: bytesType}
	str   = cType{GoName: "string", Size: 16, Align: 8, Kind: stringType}
)

type cABI struct {
	short  cType
	ushort cType
	int    cType
	uint   cType
	long   cType
	ulong  cType
	size   cType
	ptr    cType
}

func abiFor(goos, arch string) cABI {
	abi := cABI{
		short:  i16,
		ushort: u16,
		int:    i32,
		uint:   u32,
		long:   i64,
		ulong:  u64,
		size:   pointerSized("uint", false),
		ptr:    pointerSized("unsafe.Pointer", true),
	}
	if goos == "windows" && (arch == "amd64" || arch == "arm64") {
		abi.long = i32
		abi.ulong = u32
	}
	return abi
}

func pointerSized(goName string, unsafe bool) cType {
	return cType{GoName: goName, Size: 8, Align: 8, Move: "MOVQ", Load: "MOVQ", Unsafe: unsafe}
}

func supportedTypes(abi cABI) map[string]cType {
	return map[string]cType{
		"void":                 {Kind: voidType},
		"char":                 i8,
		"signed char":          i8,
		"unsigned char":        u8,
		"short":                abi.short,
		"signed short":         abi.short,
		"unsigned short":       abi.ushort,
		"int":                  abi.int,
		"signed":               abi.int,
		"signed int":           abi.int,
		"unsigned":             abi.uint,
		"unsigned int":         abi.uint,
		"long":                 abi.long,
		"signed long":          abi.long,
		"unsigned long":        abi.ulong,
		"long long":            i64,
		"signed long long":     i64,
		"unsigned long long":   u64,
		"size_t":               abi.size,
		"const char*":          pchar(abi),
		"const unsigned char*": pchar(abi),
		"void*":                abi.ptr,
		"const void*":          abi.ptr,
	}
}

func pchar(abi cABI) cType {
	t := abi.ptr
	t.Kind = bytesPtrType
	return t
}

func parseFunctions(src, goos, arch string) ([]funcSpec, error) {
	abi := abiFor(goos, arch)
	decls := funcDecls(src)
	if len(decls) == 0 {
		return nil, fmt.Errorf("no supported C function definitions found")
	}
	funcs := make([]funcSpec, 0, len(decls))
	for _, decl := range decls {
		clean := stripAttributes(decl.text)
		if !decl.marked {
			continue
		}
		m := funcDeclRE.FindStringSubmatch(clean + " {")
		if m == nil {
			return nil, fmt.Errorf("unsupported C function declaration %q", strings.TrimSpace(decl.text))
		}
		ret, err := parseType(m[1], abi)
		if err != nil {
			return nil, err
		}
		if ret.Kind == bytesType || ret.Kind == stringType || ret.Kind == bytesPtrType {
			return nil, fmt.Errorf("%s returns char pointer; []byte/string returns are not supported", m[2])
		}
		params, err := parseParams(m[3], abi, decl.goTypes)
		if err != nil {
			return nil, fmt.Errorf("%s params: %w", m[2], err)
		}
		goName := goName(m[2])
		if decl.sig != nil {
			goName, params, ret, err = applySignature(*decl.sig, params, ret)
			if err != nil {
				return nil, fmt.Errorf("%s signature: %w", m[2], err)
			}
		}
		funcs = append(funcs, funcSpec{
			CName:  m[2],
			GoName: goName,
			Return: ret,
			Params: params,
		})
	}
	return funcs, nil
}

type funcDecl struct {
	text    string
	marked  bool
	goTypes map[string]cKind
	sig     *goSignature
}

type goSignature struct {
	name   string
	params []paramSpec
	ret    *cType
}

var typeWords = map[string]bool{
	"const": true, "signed": true, "unsigned": true, "char": true,
	"short": true, "int": true, "long": true, "void": true, "size_t": true,
}

func funcDecls(src string) []funcDecl {
	var decls []funcDecl
	var pending []string
	goTypes := make(map[string]cKind)
	var sig *goSignature
	marked := false
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if len(pending) == 0 {
			name, kind, typeDirective := c2goTypeDirective(trimmed)
			parsed, sigDirective := c2goSignatureDirective(trimmed)
			switch {
			case sigDirective:
				sig = &parsed
				marked = true
				continue
			case isC2GoMarker(trimmed):
				marked = true
				continue
			case typeDirective:
				goTypes[name] = kind
				continue
			case trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//"):
				continue
			case !strings.Contains(trimmed, "(") && !strings.Contains(trimmed, "__attribute__"):
				continue
			}
		}
		pending = append(pending, line)
		if idx := strings.Index(line, "{"); idx >= 0 {
			pending[len(pending)-1] = line[:idx]
			decls = append(decls, funcDecl{strings.Join(pending, "\n"), marked, goTypes, sig})
			pending, marked, goTypes, sig = nil, false, make(map[string]cKind), nil
		} else if strings.Contains(line, ";") {
			pending, marked, goTypes, sig = nil, false, make(map[string]cKind), nil
		}
	}
	return decls
}

func isC2GoMarker(line string) bool {
	return line == "//go:c2go" || line == "// c2go"
}

func c2goTypeDirective(line string) (string, cKind, bool) {
	line = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "//go:c2go"), "// c2go"))
	fields := strings.Fields(line)
	if len(fields) != 2 {
		return "", scalarType, false
	}
	switch fields[0] {
	case "string":
		return sanitizeIdent(fields[1]), stringType, true
	case "[]byte":
		return sanitizeIdent(fields[1]), bytesType, true
	default:
		return "", scalarType, false
	}
}

func c2goSignatureDirective(line string) (goSignature, bool) {
	body, ok := c2goDirectiveBody(line)
	if !ok || !strings.HasPrefix(body, "func ") {
		return goSignature{}, false
	}
	sig, err := parseGoSignature(strings.TrimSpace(strings.TrimPrefix(body, "func ")))
	return sig, err == nil
}

func c2goDirectiveBody(line string) (string, bool) {
	for _, prefix := range []string{"//go:c2go", "// c2go"} {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix)), true
		}
	}
	return "", false
}

func stripAttributes(decl string) string {
	var out strings.Builder
	for i := 0; i < len(decl); {
		end := -1
		if strings.HasPrefix(decl[i:], "__attribute__") {
			end = attributeEnd(decl, i+len("__attribute__"))
		}
		if end < 0 {
			out.WriteByte(decl[i])
			i++
		} else {
			out.WriteByte(' ')
			i = end
		}
	}
	return out.String()
}

func attributeEnd(s string, start int) int {
	i := start
	for i < len(s) && strings.ContainsRune(" \t\n\r", rune(s[i])) {
		i++
	}
	if i >= len(s) || s[i] != '(' {
		return -1
	}
	depth, inString, escaped := 0, false, false
	for ; i < len(s); i++ {
		ch := s[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return -1
}

func parseParams(text string, abi cABI, goTypes map[string]cKind) ([]paramSpec, error) {
	text = strings.TrimSpace(text)
	if text == "" || text == "void" {
		return nil, nil
	}
	raw := make([]paramSpec, 0, strings.Count(text, ",")+1)
	for i, part := range strings.Split(text, ",") {
		p, err := parseRawParam(part, i, abi)
		if err != nil {
			return nil, err
		}
		raw = append(raw, p)
	}
	if len(raw) == 1 && raw[0].Type.Kind == voidType {
		return nil, nil
	}
	params := make([]paramSpec, 0, len(raw))
	usedTypes := make(map[string]bool, len(goTypes))
	for i := 0; i < len(raw); i++ {
		p := raw[i]
		if p.Type.Kind == voidType {
			return nil, fmt.Errorf("void must be the only parameter")
		}
		kind, hasType := goTypes[p.Name]
		if hasType && p.Type.Kind != bytesPtrType {
			return nil, fmt.Errorf("go type directive for %q requires a char pointer parameter", p.Name)
		}
		if p.Type.Kind == bytesPtrType {
			if i+1 >= len(raw) || raw[i+1].Type != abi.size {
				return nil, fmt.Errorf("char pointer parameter %q requires a following size_t length parameter", p.Name)
			}
			usedTypes[p.Name] = hasType
			if kind == stringType {
				p.Type = str
			} else {
				p.Type = bytes
			}
			i++
		}
		params = append(params, p)
	}
	for name := range goTypes {
		if !usedTypes[name] {
			return nil, fmt.Errorf("go type directive references unknown char pointer parameter %q", name)
		}
	}
	return params, nil
}

func parseRawParam(part string, index int, abi cABI) (paramSpec, error) {
	fields := strings.Fields(strings.ReplaceAll(strings.TrimSpace(part), "*", " * "))
	if len(fields) == 0 {
		return paramSpec{}, fmt.Errorf("unsupported parameter %q", part)
	}
	name, typeFields := fmt.Sprintf("p%d", index), fields
	if last := fields[len(fields)-1]; isIdent(last) && last != "*" && !typeWords[last] {
		name, typeFields = sanitizeIdent(last), fields[:len(fields)-1]
	}
	typ, err := parseType(strings.Join(typeFields, " "), abi)
	return paramSpec{Name: name, Type: typ}, err
}

func parseType(name string, abi cABI) (cType, error) {
	name = normalizeType(name)
	if typ, ok := supportedTypes(abi)[name]; ok {
		return typ, nil
	}
	return cType{}, fmt.Errorf("unsupported C type %q", name)
}

func parseGoSignature(text string) (goSignature, error) {
	decl, err := parser.ParseFile(token.NewFileSet(), "sig.go", "package p\nfunc "+text+" {}\n", 0)
	if err != nil {
		return goSignature{}, err
	}
	if len(decl.Decls) != 1 {
		return goSignature{}, fmt.Errorf("unsupported Go signature %q", text)
	}
	fn, ok := decl.Decls[0].(*ast.FuncDecl)
	if !ok || fn.Recv != nil || fn.Name == nil {
		return goSignature{}, fmt.Errorf("unsupported Go signature %q", text)
	}
	params, err := parseGoFields(fn.Type.Params)
	if err != nil {
		return goSignature{}, err
	}
	results, err := parseGoFields(fn.Type.Results)
	if err != nil {
		return goSignature{}, err
	}
	if len(results) > 1 {
		return goSignature{}, fmt.Errorf("multiple Go return values are not supported")
	}
	var ret *cType
	if len(results) == 1 {
		ret = &results[0].Type
	}
	return goSignature{name: fn.Name.Name, params: params, ret: ret}, nil
}

func parseGoFields(list *ast.FieldList) ([]paramSpec, error) {
	if list == nil {
		return nil, nil
	}
	var out []paramSpec
	for _, field := range list.List {
		t, err := parseGoType(field.Type)
		if err != nil {
			return nil, err
		}
		if len(field.Names) == 0 {
			out = append(out, paramSpec{Type: t})
			continue
		}
		for _, name := range field.Names {
			out = append(out, paramSpec{Name: sanitizeIdent(name.Name), Type: t})
		}
	}
	return out, nil
}

func parseGoType(expr ast.Expr) (cType, error) {
	switch expr := expr.(type) {
	case *ast.Ident:
		return parseGoTypeName(expr.Name)
	case *ast.SelectorExpr:
		if pkg, ok := expr.X.(*ast.Ident); ok && pkg.Name == "unsafe" && expr.Sel.Name == "Pointer" {
			return pointerSized("unsafe.Pointer", true), nil
		}
	case *ast.ArrayType:
		if expr.Len == nil {
			if elem, ok := expr.Elt.(*ast.Ident); ok && elem.Name == "byte" {
				return bytes, nil
			}
		}
	}
	return cType{}, fmt.Errorf("unsupported Go type %s", goExprString(expr))
}

func parseGoTypeName(name string) (cType, error) {
	switch name {
	case "int8":
		return i8, nil
	case "uint8":
		return u8, nil
	case "int16":
		return i16, nil
	case "uint16":
		return u16, nil
	case "int32":
		return i32, nil
	case "uint32":
		return u32, nil
	case "int64":
		return i64, nil
	case "uint64":
		return u64, nil
	case "uint":
		return pointerSized("uint", false), nil
	case "string":
		return str, nil
	default:
		return cType{}, fmt.Errorf("unsupported Go type %q", name)
	}
}

func goExprString(expr ast.Expr) string {
	var b strings.Builder
	ast.Fprint(&b, nil, expr, nil)
	return b.String()
}

func applySignature(sig goSignature, params []paramSpec, ret cType) (string, []paramSpec, cType, error) {
	if len(sig.params) != len(params) {
		return "", nil, cType{}, fmt.Errorf("parameter count mismatch: got %d, want %d", len(sig.params), len(params))
	}
	for i := range params {
		if !compatibleGoType(params[i].Type, sig.params[i].Type) {
			return "", nil, cType{}, fmt.Errorf("parameter %q has incompatible Go type %s", params[i].Name, sig.params[i].Type.GoName)
		}
		params[i].Name = sig.params[i].Name
		params[i].Type = sig.params[i].Type
	}
	if sig.ret == nil {
		if ret.Kind != voidType {
			return "", nil, cType{}, fmt.Errorf("missing Go return type")
		}
	} else {
		if ret.Kind == voidType {
			return "", nil, cType{}, fmt.Errorf("unexpected Go return type %s", sig.ret.GoName)
		}
		if !compatibleGoType(ret, *sig.ret) {
			return "", nil, cType{}, fmt.Errorf("incompatible Go return type %s", sig.ret.GoName)
		}
		ret = *sig.ret
	}
	return sig.name, params, ret, nil
}

func compatibleGoType(inferred, explicit cType) bool {
	if inferred.Kind == bytesType && explicit.Kind == stringType || inferred.Kind == stringType && explicit.Kind == bytesType {
		return true
	}
	return inferred == explicit
}

func normalizeType(name string) string {
	name = strings.ReplaceAll(name, "*", " * ")
	name = strings.Join(strings.Fields(name), " ")
	name = strings.ReplaceAll(name, " *", "*")
	return strings.ReplaceAll(name, "* ", "*")
}

func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || i > 0 && r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}

func cArgCount(params []paramSpec) int {
	n := 0
	for _, p := range params {
		n++
		if p.Type.Kind == bytesType || p.Type.Kind == stringType {
			n++
		}
	}
	return n
}
