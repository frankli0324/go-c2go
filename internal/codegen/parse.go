package codegen

import (
	"fmt"
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
	Void   bool
	Bytes  bool
	Unsafe bool
}

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
	bytes = cType{GoName: "[]byte", Size: 24, Align: 8, Bytes: true}
	ptr   = cType{GoName: "unsafe.Pointer", Size: 8, Align: 8, Move: "MOVQ", Load: "MOVQ", Unsafe: true}
)

type cABI struct {
	short  cType
	ushort cType
	int    cType
	uint   cType
	long   cType
	ulong  cType
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
		ptr:    ptr,
	}
	if goos == "windows" && (arch == "amd64" || arch == "arm64") {
		abi.long = i32
		abi.ulong = u32
	}
	return abi
}

func supportedTypes(abi cABI) map[string]cType {
	return map[string]cType{
		"void":               {Void: true},
		"char":               i8,
		"signed char":        i8,
		"unsigned char":      u8,
		"short":              abi.short,
		"signed short":       abi.short,
		"unsigned short":     abi.ushort,
		"int":                abi.int,
		"signed":             abi.int,
		"signed int":         abi.int,
		"unsigned":           abi.uint,
		"unsigned int":       abi.uint,
		"long":               abi.long,
		"signed long":        abi.long,
		"unsigned long":      abi.ulong,
		"long long":          i64,
		"signed long long":   i64,
		"unsigned long long": u64,
		"const char*":        bytes,
		"void*":              abi.ptr,
		"const void*":        abi.ptr,
	}
}

func parseFunctions(src, goos, arch string) ([]funcSpec, error) {
	abi := abiFor(goos, arch)
	decls := funcDecls(src)
	if len(decls) == 0 {
		return nil, fmt.Errorf("no supported C function definitions found")
	}
	marked := false
	for _, decl := range decls {
		if decl.marked {
			marked = true
			break
		}
	}

	funcs := make([]funcSpec, 0, len(decls))
	for _, decl := range decls {
		clean := stripAttributes(decl.text)
		if marked && !decl.marked {
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
		if ret.Bytes {
			return nil, fmt.Errorf("%s returns const char *; []byte returns are not supported", m[2])
		}
		params, err := parseParams(m[3], abi)
		if err != nil {
			return nil, fmt.Errorf("%s params: %w", m[2], err)
		}
		funcs = append(funcs, funcSpec{
			CName:  m[2],
			GoName: goName(m[2]),
			Return: ret,
			Params: params,
		})
	}
	return funcs, nil
}

type funcDecl struct {
	text   string
	marked bool
}

var typeWords = map[string]bool{
	"const": true, "signed": true, "unsigned": true, "char": true,
	"short": true, "int": true, "long": true, "void": true,
}

func funcDecls(src string) []funcDecl {
	var decls []funcDecl
	var pending []string
	marked := false
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if len(pending) == 0 {
			switch {
			case isC2GoMarker(trimmed):
				marked = true
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
			decls = append(decls, funcDecl{strings.Join(pending, "\n"), marked})
			pending, marked = nil, false
		} else if strings.Contains(line, ";") {
			pending, marked = nil, false
		}
	}
	return decls
}

func isC2GoMarker(line string) bool {
	return line == "//go:c2go" || line == "// c2go"
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

func parseParams(text string, abi cABI) ([]paramSpec, error) {
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
	if len(raw) == 1 && raw[0].Type.Void {
		return nil, nil
	}
	params := make([]paramSpec, 0, len(raw))
	for i := 0; i < len(raw); i++ {
		p := raw[i]
		if p.Type.Void {
			return nil, fmt.Errorf("void must be the only parameter")
		}
		if p.Type.Bytes {
			if i+1 >= len(raw) || !sameType(raw[i+1].Type, abi.int) {
				return nil, fmt.Errorf("[]byte parameter %q requires a following int length parameter", p.Name)
			}
			i++
		}
		params = append(params, p)
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

func normalizeType(name string) string {
	name = strings.ReplaceAll(name, "*", " * ")
	name = strings.Join(strings.Fields(name), " ")
	name = strings.ReplaceAll(name, " *", "*")
	return strings.ReplaceAll(name, "* ", "*")
}

func sameType(a, b cType) bool {
	return a.GoName == b.GoName && a.Size == b.Size && a.Void == b.Void && a.Bytes == b.Bytes && a.Unsafe == b.Unsafe
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
		if p.Type.Bytes {
			n++
		}
	}
	return n
}
