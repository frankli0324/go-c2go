package arm64

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/frankli0324/go-c2go/internal/asm/asmutil"
)

func (t *Translator) load(op string, args []string) (string, error) {
	if len(args) != 2 && len(args) != 3 {
		return "", fmt.Errorf("unsupported arm64 load")
	}
	if len(args) == 3 {
		base := strings.TrimSuffix(strings.TrimSpace(args[1]), "]")
		mem, err := t.memory(base + ", " + args[2] + "]")
		if err != nil {
			return "", err
		}
		dst, err := operand(args[0])
		if err != nil {
			return "", err
		}
		return loadMnemonic(op, args[0]) + ".P " + mem + ", " + dst, nil
	}
	if isFloatReg(args[0]) {
		dst, err := floatRegister(args[0])
		if err != nil {
			return "", err
		}
		mem, err := t.memory(args[1])
		if err != nil {
			return "", err
		}
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(args[0])), "q") {
			return "FMOVQ " + mem + ", " + dst, nil
		}
		return "FMOVD " + mem + ", " + dst, nil
	}
	dst, err := operand(args[0])
	if err != nil {
		return "", err
	}
	mnemonic := loadMnemonic(op, args[0])
	memArg := strings.TrimSpace(args[1])
	if strings.HasSuffix(memArg, "]!") {
		memArg = strings.TrimSuffix(memArg, "!")
		mnemonic += ".W"
	}
	mem, err := t.memory(memArg)
	if err != nil {
		return "", err
	}
	t.clear(args[0])
	return mnemonic + " " + mem + ", " + dst, nil
}

func (t *Translator) store(op string, args []string) (string, error) {
	if len(args) != 2 {
		return "", fmt.Errorf("unsupported arm64 store")
	}
	if isFloatReg(args[0]) {
		src, err := floatRegister(args[0])
		if err != nil {
			return "", err
		}
		memArg := strings.TrimSpace(args[1])
		mnemonic := "FMOVD"
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(args[0])), "q") {
			mnemonic = "FMOVQ"
		}
		if strings.HasSuffix(memArg, "]!") {
			memArg = strings.TrimSuffix(memArg, "!")
			mnemonic += ".W"
		}
		mem, err := t.memory(memArg)
		if err != nil {
			return "", err
		}
		return mnemonic + " " + src + ", " + mem, nil
	}
	src, err := operand(args[0])
	if err != nil {
		return "", err
	}
	mnemonic := storeMnemonic(op, args[0])
	memArg := strings.TrimSpace(args[1])
	if strings.HasSuffix(memArg, "]!") {
		memArg = strings.TrimSuffix(memArg, "!")
		mnemonic += ".W"
	}
	mem, err := t.memory(memArg)
	if err != nil {
		return "", err
	}
	return mnemonic + " " + src + ", " + mem, nil
}

func loadMnemonic(op, dst string) string {
	switch {
	case strings.Contains(op, "rsb"):
		return "MOVB"
	case strings.Contains(op, "rsh"):
		return "MOVH"
	case strings.Contains(op, "rsw"):
		return "MOVW"
	case strings.HasSuffix(op, "b"):
		return "MOVBU"
	case strings.HasSuffix(op, "h"):
		return "MOVHU"
	case strings.HasPrefix(strings.ToLower(strings.TrimSpace(dst)), "w"):
		return "MOVWU"
	default:
		return "MOVD"
	}
}

func storeMnemonic(op, src string) string {
	switch {
	case strings.HasSuffix(op, "b"):
		return "MOVB"
	case strings.HasSuffix(op, "h"):
		return "MOVH"
	case strings.HasPrefix(strings.ToLower(strings.TrimSpace(src)), "w"):
		return "MOVW"
	default:
		return "MOVD"
	}
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
	if len(name) < 2 || (name[0] != 'x' && name[0] != 'w') {
		return "", fmt.Errorf("unsupported arm64 register %q", name)
	}
	n, err := strconv.Atoi(name[1:])
	if err != nil || n < 0 || n > 30 {
		return "", fmt.Errorf("unsupported arm64 register %q", name)
	}
	return fmt.Sprintf("R%d", n), nil
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
	}
	return false
}

func reservedRegNumber(name string) (int, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if len(name) < 2 || (name[0] != 'x' && name[0] != 'w') {
		return 0, false
	}
	n, err := strconv.Atoi(name[1:])
	if err != nil || !reservedRegister(n) {
		return 0, false
	}
	return n, true
}

func pairReservedRegister(name string) (string, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "x26", "w26": // Allow clang's R26 stack save-restore pair to translate before inline filtering.
		return "R26", nil
	case "x27", "w27": // Allow clang's R27 stack save-restore pair to translate before inline filtering.
		return "R27", nil
	case "x28", "w28": // Allow clang's R28 stack save-restore pair to translate before inline filtering.
		return "R28", nil
	case "x29", "w29": // Allow clang's R29/R30 frame-pointer/link-register save-restore pair.
		return "R29", nil
	default:
		return "", fmt.Errorf("reserved arm64 register %q", name)
	}
}

func (t *Translator) memory(arg string) (string, error) {
	arg = strings.TrimSpace(arg)
	if !strings.HasPrefix(arg, "[") || !strings.HasSuffix(arg, "]") {
		return "", fmt.Errorf("unsupported arm64 memory %q", arg)
	}
	parts := asmutil.SplitOperands(strings.TrimSpace(arg[1 : len(arg)-1]))
	if len(parts) == 0 || len(parts) > 2 {
		return "", fmt.Errorf("unsupported arm64 memory %q", arg)
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
		return "(" + base + ")(" + reg + ")", nil
	}
	return strings.TrimPrefix(strings.TrimSpace(parts[1]), "#") + "(" + base + ")", nil
}

func (t *Translator) pairMemory(args []string) (string, string, error) {
	if len(args) != 1 && len(args) != 2 {
		return "", "", fmt.Errorf("unsupported arm64 pair memory")
	}
	memArg := args[0]
	if len(args) == 2 {
		memArg = strings.TrimSuffix(memArg, "]") + ", " + args[1] + "]"
		mem, err := t.memory(memArg)
		return mem, ".P", err
	}
	if strings.HasSuffix(strings.TrimSpace(memArg), "]!") {
		memArg = strings.TrimSuffix(strings.TrimSpace(memArg), "!")
		mem, err := t.memory(memArg)
		return mem, ".W", err
	}
	mem, err := t.memory(memArg)
	return mem, "", err
}

func isFloatReg(arg string) bool {
	arg = strings.ToLower(strings.TrimSpace(arg))
	return strings.HasPrefix(arg, "q") || strings.HasPrefix(arg, "d")
}
