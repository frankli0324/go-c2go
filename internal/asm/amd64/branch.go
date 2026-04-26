package amd64

import (
	"strings"

	"github.com/frankli0324/go-c2go/internal/asm/asmutil"
)

func convertBranchTarget(op, target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return target
	}
	if strings.HasPrefix(target, "*") {
		target = strings.TrimSpace(strings.TrimPrefix(target, "*"))
		if converted, ok := convertIndirectBranchTarget(target); ok {
			return converted
		}
		return target
	}
	if converted, ok := convertIndirectBranchTarget(target); ok {
		return converted
	}
	if asmutil.IsLocalLabel(target) || isConditionalBranch(op) {
		return target
	}
	return asmutil.AddSB(target)
}

func convertIndirectBranchTarget(target string) (string, bool) {
	if reg, err := plan9Register(target); err == nil {
		return reg, true
	}
	if strings.HasPrefix(target, "[") && strings.HasSuffix(target, "]") {
		if mem, err := convertIntelMemory(target); err == nil {
			return mem, true
		}
	}
	if strings.Contains(target, "(") && strings.HasSuffix(target, ")") {
		if mem, err := convertATTMemory(target); err == nil {
			return mem, true
		}
	}
	return "", false
}

func isConditionalBranch(op string) bool {
	lower := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(op)), "q")
	if lower == "call" || lower == "jmp" || lower == "ret" {
		return false
	}
	_, ok := opSpecs[lower]
	return ok
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
