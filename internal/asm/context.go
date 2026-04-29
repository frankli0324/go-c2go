package asm

import (
	"strconv"
	"strings"
)

func (ctx Context) supportsPCALIGN() bool {
	if ctx.Arch != ArchAMD64 {
		return true
	}
	major, minor, ok := parseGoVersion(ctx.GoVersion)
	if !ok {
		return true
	}
	return major > 1 || major == 1 && minor >= 22
}

func parseGoVersion(v string) (int, int, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "go")
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return 0, 0, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	minor, err := strconv.Atoi(parts[1])
	return major, minor, err == nil
}
