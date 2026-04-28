package amd64

import (
	"fmt"
	"strings"

	"github.com/frankli0324/go-c2go/internal/asm/asmutil"
)

func convertBranchTarget(target string, addSymbol bool) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return target, nil
	}
	indirect := strings.HasPrefix(target, "*")
	if indirect {
		target = strings.TrimSpace(strings.TrimPrefix(target, "*"))
	}
	if containsELFReloc(target) {
		return "", fmt.Errorf("unsupported ELF relocation in branch target %q", target)
	}
	if converted, ok := convertIndirectBranchTarget(target); ok {
		return converted, nil
	}
	if indirect || asmutil.IsLocalLabel(target) || !addSymbol {
		return target, nil
	}
	return asmutil.AddSB(target), nil
}

func convertIndirectBranchTarget(target string) (string, bool) {
	if reg, err := plan9Register(target); err == nil {
		return reg, true
	}
	if strings.HasPrefix(target, "[") && strings.HasSuffix(target, "]") {
		if mem, err := convertIntelMemory(target); err == nil {
			return strings.TrimPrefix(mem, "$"), true
		}
	}
	if strings.Contains(target, "(") && strings.HasSuffix(target, ")") {
		if mem, err := convertATTMemory(target); err == nil {
			return strings.TrimPrefix(mem, "$"), true
		}
	}
	return "", false
}

func cmovMnemonic(op string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(op))
	if !strings.HasPrefix(lower, "cmov") || len(lower) < len("cmovxq") {
		return "", false
	}
	width := lower[len(lower)-1]
	if !strings.ContainsRune("wlq", rune(width)) {
		return "", false
	}
	cond, ok := conditionSuffix(lower[len("cmov") : len(lower)-1])
	if !ok {
		return "", false
	}
	return "CMOV" + strings.ToUpper(string(width)) + cond, true
}

func setCCMnemonic(op string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(op))
	if !strings.HasPrefix(lower, "set") || len(lower) <= len("set") {
		return "", false
	}
	cond, ok := conditionSuffix(lower[len("set"):])
	if !ok {
		return "", false
	}
	return "SET" + cond, true
}

func conditionSuffix(cond string) (string, bool) {
	switch cond {
	case "e", "z":
		return "EQ", true
	case "ne", "nz":
		return "NE", true
	case "g", "nle":
		return "GT", true
	case "ge", "nl":
		return "GE", true
	case "l", "nge":
		return "LT", true
	case "le", "ng":
		return "LE", true
	case "a", "nbe":
		return "HI", true
	case "ae", "nb", "nc":
		return "CC", true
	case "b", "c", "nae":
		return "CS", true
	case "be", "na":
		return "LS", true
	case "o":
		return "OS", true
	case "no":
		return "OC", true
	case "s":
		return "MI", true
	case "ns":
		return "PL", true
	case "p", "pe":
		return "PS", true
	case "np", "po":
		return "PC", true
	default:
		return "", false
	}
}
