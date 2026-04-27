package amd64

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/frankli0324/go-c2go/internal/asm/asmutil"
)

var attMemoryRE = regexp.MustCompile(`^(.*)\(%?([^,]*)(?:,%([^,]*)(?:,(\d+))?)?\)$`)

type ATT struct{}

func (ATT) CommentPrefix() string {
	return "#"
}

func (ATT) TranslateInstruction(indent, line string) (string, bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return indent, false
	}
	op := fields[0]
	argsText := strings.TrimSpace(strings.TrimPrefix(line, op))
	out, err := translateAMD64(op, asmutil.SplitOperands(argsText), syntaxATT, attHandlers)
	if err != nil {
		return indent + "// UNSUPPORTED: " + line, true
	}
	return indent + out, false
}

var attHandlers = map[opType]opHandler{
	opFixed:      fixedHandler,
	opBranch:     branchHandler,
	opSIMDExact:  simdExactHandler,
	opSIMDSuffix: simdSuffixHandler,
	opATTSized:   attSizedHandler,
	opCMOV:       attCMOVHandler,
	opSETCC:      setCCHandler,
}

func attSizedHandler(ctx opContext) (string, []string, error) {
	if len(ctx.op) > 1 && strings.ContainsRune("bwlq", rune(ctx.op[len(ctx.op)-1])) {
		ops, err := convertOperands(ctx)
		suffix := ctx.op[len(ctx.op)-1:]
		mnemonic := strings.ToUpper(ctx.op)
		if ctx.op[:len(ctx.op)-1] == "imul" && len(ctx.ops) == 3 {
			mnemonic = "IMUL3" + strings.ToUpper(suffix)
		}
		return mnemonic, ops, err
	}
	return "", nil, fmt.Errorf("unsupported mnemonic %q", ctx.op)
}

func attCMOVHandler(ctx opContext) (string, []string, error) {
	if mnemonic, ok := cmovMnemonic(ctx.op); ok {
		ops, err := convertOperands(ctx)
		return mnemonic, ops, err
	}
	return "", nil, fmt.Errorf("unsupported cmov mnemonic %q", ctx.op)
}

func convertATTOperand(op, arg string) (string, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "", fmt.Errorf("empty operand")
	}
	if strings.HasPrefix(arg, "%") {
		return plan9Register(strings.TrimPrefix(arg, "%"))
	}
	if strings.HasPrefix(arg, "$") {
		value := strings.TrimPrefix(arg, "$")
		if strings.EqualFold(op, "leaq") && !asmutil.IsNumeric(value) {
			return asmutil.AddSB(value), nil
		}
		return plan9Immediate(value), nil
	}
	if strings.Contains(arg, "(") && strings.HasSuffix(arg, ")") {
		converted, err := convertATTMemory(arg)
		if err != nil {
			return "", err
		}
		if strings.EqualFold(op, "leaq") && strings.HasPrefix(converted, "$") {
			converted = strings.TrimPrefix(converted, "$")
		}
		return converted, nil
	}
	return arg, nil
}

func convertATTMemory(arg string) (string, error) {
	m := attMemoryRE.FindStringSubmatch(arg)
	if m == nil {
		return "", fmt.Errorf("unsupported memory operand %q", arg)
	}
	disp := strings.TrimSpace(m[1])
	base := strings.TrimSpace(m[2])
	index := strings.TrimSpace(m[3])
	scale := strings.TrimSpace(m[4])
	if strings.EqualFold(base, "rip") {
		return plan9Immediate(disp), nil
	}
	baseReg := ""
	if base != "" {
		reg, err := plan9Register(base)
		if err != nil {
			return "", err
		}
		baseReg = reg
	}
	indexText := ""
	if index != "" {
		reg, err := plan9Register(index)
		if err != nil {
			return "", err
		}
		if scale == "" {
			scale = "1"
		}
		indexText = "(" + reg + "*" + scale + ")"
	}
	if baseReg == "" {
		if indexText != "" {
			return disp + indexText, nil
		}
		return plan9Immediate(disp), nil
	}
	return disp + "(" + baseReg + ")" + indexText, nil
}

func reorderATTOperands(op string, operands []string) []string {
	if len(operands) == 1 && isUnaryOneShift(op) {
		return []string{"$1", operands[0]}
	}
	if len(operands) == 2 && strings.HasPrefix(strings.ToLower(op), "cmp") && strings.HasPrefix(operands[0], "$") {
		return []string{operands[1], operands[0]}
	}
	return operands
}

func isUnaryOneShift(op string) bool {
	op = strings.ToLower(op)
	if len(op) > 1 && strings.ContainsRune("bwlq", rune(op[len(op)-1])) {
		op = op[:len(op)-1]
	}
	switch op {
	case "shl", "shr", "sal", "sar", "rol", "ror":
		return true
	default:
		return false
	}
}
