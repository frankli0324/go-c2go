package codegen

import "unicode"

func CompilerSymbol(goos, name string) string {
	return compilerSymbol(goos, name)
}

func compilerSymbol(goos, name string) string {
	if goos == "darwin" {
		return "_" + name
	}
	return name
}

func rawSymbol(name string) string {
	return "c2go_" + sanitizeIdent(name)
}

func goName(name string) string {
	var out []rune
	upperNext := true
	for _, r := range name {
		if r == '_' {
			upperNext = true
			continue
		}
		if upperNext {
			out = append(out, unicode.ToUpper(r))
			upperNext = false
			continue
		}
		out = append(out, r)
	}
	if len(out) == 0 || !unicode.IsLetter(out[0]) {
		return "Func"
	}
	return string(out)
}

func sanitizeIdent(name string) string {
	var out []rune
	for i, r := range name {
		if r == '_' || unicode.IsLetter(r) || (i > 0 && unicode.IsDigit(r)) {
			out = append(out, r)
		}
	}
	if len(out) == 0 || out[0] == '_' || unicode.IsDigit(out[0]) {
		return "x" + string(out)
	}
	return string(out)
}
