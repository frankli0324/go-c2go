package arm64

import "strings"

func pageSymbol(arg string) string {
	arg = strings.TrimSpace(arg)
	if i := strings.Index(arg, "@PAGE"); i >= 0 {
		arg = arg[:i] + arg[i+len("@PAGE"):]
	}
	if sym, ok := pageOffsetSymbol(arg); ok {
		return sym
	}
	return arg
}

func pageOffsetSymbol(arg string) (string, bool) {
	arg = strings.TrimPrefix(strings.TrimSpace(arg), "#")
	if strings.HasPrefix(arg, ":lo12:") {
		return strings.TrimPrefix(arg, ":lo12:"), true
	}
	if i := strings.Index(arg, "@PAGEOFF"); i >= 0 {
		return arg[:i] + arg[i+len("@PAGEOFF"):], true
	}
	return "", false
}
