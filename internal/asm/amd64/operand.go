package amd64

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/frankli0324/go-c2go/internal/asm/asmutil"
)

func plan9Register(name string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, "%")))
	if key == "" {
		return "", fmt.Errorf("empty register")
	}
	if reg, ok := legacyRegister(key); ok {
		return reg, nil
	}
	if reg, ok := numberedRegister(key); ok {
		return reg, nil
	}
	return "", fmt.Errorf("unsupported register %q", name)
}

func legacyRegister(key string) (string, bool) {
	if key == "rip" {
		return "PC", true
	}
	for _, group := range legacyRegisterGroups {
		for _, name := range strings.Fields(group.names) {
			if key == name {
				return group.reg, true
			}
		}
	}
	return "", false
}

func intelRegWidth(arg string) (string, bool) {
	key := strings.ToLower(strings.TrimSpace(arg))
	if _, suffix, ok := numberedGPR(key); ok {
		return widthFromSuffix(suffix)
	}
	for _, group := range widthRegisterGroups {
		if strings.Contains(" "+group.names+" ", " "+key+" ") {
			return group.width, true
		}
	}
	return "", false
}

var legacyRegisterGroups = []struct{ reg, names string }{
	{"AL", "al"},
	{"BL", "bl"},
	{"CL", "cl"},
	{"DL", "dl"},
	{"SPB", "spl"},
	{"BPB", "bpl"},
	{"SIB", "sil"},
	{"DIB", "dil"},
	{"AH", "ah"},
	{"BH", "bh"},
	{"CH", "ch"},
	{"DH", "dh"},
	{"AX", "rax eax ax"},
	{"BX", "rbx ebx bx"},
	{"CX", "rcx ecx cx"},
	{"DX", "rdx edx dx"},
	{"SI", "rsi esi si"},
	{"DI", "rdi edi di"},
	{"BP", "rbp ebp bp"},
	{"SP", "rsp esp sp"},
}

var widthRegisterGroups = []struct{ width, names string }{
	{"Q", "rax rbx rcx rdx rsi rdi rbp rsp"},
	{"L", "eax ebx ecx edx esi edi ebp esp"},
	{"W", "ax bx cx dx si di bp sp"},
	{"B", "al bl cl dl sil dil bpl spl ah bh ch dh"},
}

func numberedRegister(key string) (string, bool) {
	if n, prefix, ok := simdRegister(key); ok {
		return prefix + strconv.Itoa(n), true
	}
	if n, suffix, ok := numberedGPR(key); ok {
		if suffix == "b" {
			return "R" + strconv.Itoa(n) + "B", true
		}
		return "R" + strconv.Itoa(n), true
	}
	return "", false
}

func simdRegister(key string) (int, string, bool) {
	for _, spec := range []struct {
		asm, plan9 string
		max        int
	}{{"xmm", "X", 31}, {"ymm", "Y", 31}, {"zmm", "Z", 31}, {"mm", "M", 7}, {"k", "K", 7}} {
		if strings.HasPrefix(key, spec.asm) {
			n, err := strconv.Atoi(strings.TrimPrefix(key, spec.asm))
			return n, spec.plan9, err == nil && n >= 0 && n <= spec.max
		}
	}
	return 0, "", false
}

func numberedGPR(key string) (int, string, bool) {
	if !strings.HasPrefix(key, "r") || len(key) < 2 {
		return 0, "", false
	}
	rest := strings.TrimPrefix(key, "r")
	digits := strings.TrimRight(rest, "dwb")
	n, err := strconv.Atoi(digits)
	suffix := strings.TrimPrefix(rest, digits)
	return n, suffix, err == nil && n >= 8 && n <= 15 && (suffix == "" || suffix == "d" || suffix == "w" || suffix == "b")
}

func widthFromSuffix(suffix string) (string, bool) {
	switch suffix {
	case "":
		return "Q", true
	case "d":
		return "L", true
	case "w":
		return "W", true
	case "b":
		return "B", true
	default:
		return "", false
	}
}

func plan9Immediate(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "+")
	if v == "" {
		return "$0"
	}
	if asmutil.IsNumeric(v) {
		return "$" + v
	}
	return "$" + asmutil.AddSB(v)
}

func containsELFReloc(s string) bool {
	return strings.Contains(s, "@")
}

func isIntelImmediate(v string) bool {
	v = strings.TrimSpace(v)
	if asmutil.IsNumeric(v) {
		return true
	}
	return !strings.ContainsAny(v, " [](),%")
}
