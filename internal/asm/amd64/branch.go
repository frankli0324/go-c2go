package amd64

import (
	"fmt"
	"strings"

	"github.com/frankli0324/go-c2go/internal/asm/asmutil"
	"golang.org/x/arch/x86/x86asm"
)

var amd64Conditions = map[string]string{}

func init() {
	add := func(branch x86asm.Op, suffix string, names ...string) {
		for _, name := range names {
			amd64Conditions[name] = suffix
			opSpecs["j"+name] = opSpec{typ: opTarget, mn: branch.String()}
		}
	}
	add(x86asm.JE, "EQ", "e", "z")
	add(x86asm.JNE, "NE", "ne", "nz")
	add(x86asm.JG, "GT", "g")
	add(x86asm.JGE, "GE", "ge")
	add(x86asm.JL, "LT", "l")
	add(x86asm.JLE, "LE", "le")
	add(x86asm.JA, "HI", "a")
	add(x86asm.JAE, "CC", "ae", "nc")
	add(x86asm.JB, "CS", "b", "c")
	add(x86asm.JBE, "LS", "be")
	add(x86asm.JO, "OS", "o")
	add(x86asm.JNO, "OC", "no")
	add(x86asm.JS, "MI", "s")
	add(x86asm.JNS, "PL", "ns")
	add(x86asm.JP, "PS", "p", "pe")
	add(x86asm.JNP, "PC", "np", "po")
}

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
	if indirect {
		return "", fmt.Errorf("unsupported indirect branch target %q", target)
	}
	if asmutil.IsLocalLabel(target) || !addSymbol {
		return target, nil
	}
	return asmutil.AddSB(target), nil
}

func convertIndirectBranchTarget(target string) (string, bool) {
	target, _ = stripIntelPtr(target)
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
	if suffix, ok := amd64Conditions[cond]; ok {
		return suffix, true
	}
	return "", false
}
