package arm64

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/frankli0324/go-c2go/internal/asm/asmutil"
	"golang.org/x/arch/arm64/arm64asm"
)

var exactMemMnemonics = map[string]string{
	"ldrsb": "MOVB", "ldursb": "MOVB",
	"ldrsh": "MOVH", "ldursh": "MOVH",
	"ldrsw": "MOVW", "ldursw": "MOVW",
	"ldrb": "MOVBU", "ldurb": "MOVBU",
	"ldrh": "MOVHU", "ldurh": "MOVHU",
	"strb": "MOVB", "sturb": "MOVB",
	"strh": "MOVH", "sturh": "MOVH",
}

func (t *Translator) load(op string, args []string) (string, error) {
	if len(args) != 2 && len(args) != 3 {
		return "", fmt.Errorf("unsupported arm64 load")
	}
	mnemonic, err := memMnemonic(op, args[0])
	if err != nil {
		return "", err
	}
	out, err := t.memText(true, mnemonic, args[0], args[1:])
	if err == nil {
		t.clear(args[0])
	}
	return out, err
}

func (t *Translator) store(op string, args []string) (string, error) {
	if len(args) != 2 {
		return "", fmt.Errorf("unsupported arm64 store")
	}
	mnemonic, err := memMnemonic(op, args[0])
	if err != nil {
		return "", err
	}
	return t.memText(false, mnemonic, args[0], args[1:])
}

func (t *Translator) memText(load bool, mnemonic, regArg string, memArgs []string) (string, error) {
	mem, suffix, err := t.memorySuffix(memArgs)
	if err != nil {
		return "", err
	}
	mnemonic += suffix
	reg, err := operand(regArg)
	if isFloatReg(regArg) {
		reg, err = floatRegister(regArg)
	}
	if err != nil {
		return "", err
	}
	if load {
		return mnemonic + " " + mem + ", " + reg, nil
	}
	return mnemonic + " " + reg + ", " + mem, nil
}

func memMnemonic(op, reg string) (string, error) {
	if mn, ok := exactMemMnemonics[op]; ok {
		return mn, nil
	}
	load := op == "ldr" || op == "ldur"
	if !load && op != "str" && op != "stur" {
		return "", fmt.Errorf("unsupported arm64 memory op %q %q", op, reg)
	}
	reg = strings.TrimSpace(reg)
	lowerReg := strings.ToLower(reg)
	if _, err := register(reg); err == nil {
		switch {
		case strings.HasPrefix(lowerReg, "x"):
			return "MOVD", nil
		case load && strings.HasPrefix(lowerReg, "w"):
			return "MOVWU", nil
		case strings.HasPrefix(lowerReg, "w"):
			return "MOVW", nil
		}
	}
	if _, err := floatRegister(reg); err != nil {
		return "", fmt.Errorf("unsupported arm64 memory register %q", reg)
	}
	switch lowerReg[0] {
	case 's':
		return "FMOVS", nil
	case 'd':
		return "FMOVD", nil
	case 'q':
		return "FMOVQ", nil
	}
	return "", fmt.Errorf("unsupported arm64 memory register %q", reg)
}

func operand(arg string) (string, error) {
	arg = strings.TrimSpace(arg)
	if strings.HasPrefix(arg, "#") {
		return "$" + strings.TrimPrefix(arg, "#"), nil
	}
	return register(arg)
}

func mustOperand(arg string) string {
	out, err := operand(arg)
	if err != nil {
		return arg
	}
	return out
}

func register(name string) (string, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "sp":
		return "RSP", nil
	case "xzr", "wzr":
		return "ZR", nil
	}
	if n, ok := regNumber(name); ok {
		return fmt.Sprintf("R%d", n), nil
	}
	return "", fmt.Errorf("unsupported arm64 register %q", name)
}

func asmRegister(name string) (arm64asm.Arg, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "sp":
		return arm64asm.RegSP(arm64asm.SP), nil
	case "xzr":
		return arm64asm.XZR, nil
	case "wzr":
		return arm64asm.WZR, nil
	}
	if n, ok := regNumber(name); ok {
		if name[0] == 'w' {
			return arm64asm.W0 + arm64asm.Reg(n), nil
		}
		return arm64asm.X0 + arm64asm.Reg(n), nil
	}
	return nil, fmt.Errorf("unsupported arm64 register %q", name)
}

func reservedRegNumber(name string) (int, bool) {
	if n, ok := regNumber(name); ok && reservedRegister(n) {
		return n, true
	}
	return 0, false
}

func regNumber(name string) (int, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if len(name) < 2 || name[0] != 'x' && name[0] != 'w' {
		return 0, false
	}
	n, err := strconv.Atoi(name[1:])
	return n, err == nil && n >= 0 && n <= 30
}

func reservedRegister(n int) bool {
	switch n {
	case 18: // R18 is R18_PLATFORM on Apple ARM64.
		return true
	case 26: // R26 is REGCTXT in Go ABIInternal; it carries closure context at calls.
		return true
	case 27: // R27 is reserved by the Go compiler and linker.
		return true
	case 28: // R28 is REGG in Go ABIInternal.
		return true
	case 29: // R29 is FP in Go ARM64 assembly.
		return true
	default:
		return false
	}
}

func pairReservedRegister(name string) (string, error) {
	if n, ok := regNumber(name); ok && n >= 26 && n <= 29 {
		return fmt.Sprintf("R%d", n), nil
	}
	return "", fmt.Errorf("reserved arm64 register %q", name)
}

func (t *Translator) formatMemory(parts []string) (string, error) {
	if len(parts) == 0 || len(parts) > 3 {
		return "", fmt.Errorf("unsupported arm64 memory")
	}
	base, err := register(parts[0])
	if err != nil {
		return "", err
	}
	if len(parts) == 1 {
		return "(" + base + ")", nil
	}
	if sym, ok := pageOffsetSymbol(parts[1]); ok {
		if t.fullAddr[base] == sym {
			return "(" + base + ")", nil
		}
		return "", fmt.Errorf("arm64 symbolic memory offset %q without resolved base %s", parts[1], parts[0])
	}
	if reg, err := register(parts[1]); err == nil {
		if len(parts) == 3 {
			amount, err := shift(parts[2])
			if err != nil {
				return "", err
			}
			return "(" + base + ")(" + reg + "<<" + amount + ")", nil
		}
		return "(" + base + ")(" + reg + ")", nil
	}
	if len(parts) == 3 {
		return "", fmt.Errorf("unsupported arm64 memory offset %q", strings.Join(parts[1:], ", "))
	}
	return strings.TrimPrefix(strings.TrimSpace(parts[1]), "#") + "(" + base + ")", nil
}

func memoryParts(arg string) ([]string, bool) {
	arg = strings.TrimSpace(arg)
	if strings.HasSuffix(arg, "]!") || !strings.HasPrefix(arg, "[") || !strings.HasSuffix(arg, "]") {
		return nil, false
	}
	return asmutil.SplitOperands(strings.TrimSpace(arg[1 : len(arg)-1])), true
}

func (t *Translator) memorySuffix(args []string) (string, string, error) {
	if len(args) != 1 && len(args) != 2 {
		return "", "", fmt.Errorf("unsupported arm64 memory")
	}
	if len(args) == 2 {
		parts, ok := memoryParts(args[0])
		if !ok || len(parts) != 1 {
			return "", "", fmt.Errorf("unsupported arm64 post-index memory")
		}
		mem, err := t.formatMemory(append(parts, args[1]))
		return mem, ".P", err
	}

	arg := strings.TrimSpace(args[0])
	suffix := ""
	if strings.HasSuffix(arg, "]!") {
		arg = strings.TrimSuffix(arg, "!")
		suffix = ".W"
	}
	parts, ok := memoryParts(arg)
	if !ok {
		return "", "", fmt.Errorf("unsupported arm64 memory %q", args[0])
	}
	mem, err := t.formatMemory(parts)
	return mem, suffix, err
}

func isFloatReg(arg string) bool {
	_, err := floatRegister(arg)
	return err == nil
}
