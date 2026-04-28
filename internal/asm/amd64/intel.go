package amd64

import (
	"fmt"
	"strings"

	"github.com/frankli0324/go-c2go/internal/asm/asmutil"
)

type Intel struct{}

func (Intel) CommentPrefix() string {
	return ";"
}

func (Intel) TranslateInstruction(indent, line string) (string, bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return indent, false
	}
	op := strings.ToLower(fields[0])
	argsText := strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
	args := asmutil.SplitOperands(argsText)
	if op == "cdqe" {
		return indent + "MOVLQSX AX, AX", false
	}
	ptrs := make([]string, len(args))
	ops := make([]string, len(args))
	for i, arg := range args {
		operand, ptr := stripIntelPtr(arg)
		ptrs[i] = ptr
		ops[i] = operand
	}
	spec := specFor(op)
	mnemonic, converted, err := intelHandlers[spec.typ](opContext{
		op:      op,
		args:    args,
		ptrs:    ptrs,
		spec:    spec,
		ops:     ops,
		convert: convertIntelOperand,
	})
	if err != nil {
		return indent + "// UNSUPPORTED: " + line, true
	}
	return indent + asmutil.JoinInstruction(mnemonic, converted), false
}

var intelHandlers = map[opType]opHandler{
	opFixed:            intelFixedHandler,
	opCall:             targetBranchHandler,
	opJump:             targetBranchHandler,
	opCondBranch:       targetBranchHandler,
	opCondBranchSuffix: targetBranchHandler,
	opReturn:           returnHandler,
	opSIMDExact:        intelSIMDExactHandler,
	opSIMDSuffix:       intelSIMDSuffixHandler,
	opSized:            intelSizedHandler,
	opCMOV:             intelCMOVHandler,
	opSETCC:            setCCHandler,
}

func intelFixedHandler(ctx opContext) (string, []string, error) {
	ops, err := convertOperands(ctx)
	if err != nil {
		return "", nil, err
	}
	ops, err = reorderIntelOperands(ctx.op, ops)
	return ctx.spec.mn, ops, err
}

func intelSizedHandler(ctx opContext) (string, []string, error) {
	suffix := intelSuffix(ctx)
	if suffix == "" {
		return "", nil, fmt.Errorf("cannot infer width for %q", ctx.op)
	}
	ops, err := convertOperands(ctx)
	if err != nil {
		return "", nil, err
	}
	ops, err = reorderIntelOperands(ctx.op, ops)
	mnemonic := strings.ToUpper(ctx.op) + suffix
	if ctx.op == "imul" && len(ctx.ops) == 3 {
		mnemonic = "IMUL3" + suffix
	}
	return mnemonic, ops, err
}

func intelCMOVHandler(ctx opContext) (string, []string, error) {
	suffix := intelSuffix(ctx)
	if suffix == "" {
		return "", nil, fmt.Errorf("cannot infer width for %q", ctx.op)
	}
	if mnemonic, ok := cmovMnemonic(ctx.op + strings.ToLower(suffix)); ok {
		ops, err := convertOperands(ctx)
		if err != nil {
			return "", nil, err
		}
		ops, err = reorderIntelOperands(ctx.op, ops)
		return mnemonic, ops, err
	}
	return "", nil, fmt.Errorf("unsupported cmov mnemonic %q", ctx.op)
}

func intelSIMDExactHandler(ctx opContext) (string, []string, error) {
	ops, err := convertSIMDOperands(ctx)
	if err != nil {
		return "", nil, err
	}
	ops, err = reorderIntelOperands(ctx.op, ops)
	return ctx.spec.mn, ops, err
}

func intelSIMDSuffixHandler(ctx opContext) (string, []string, error) {
	ops, err := convertSIMDOperands(ctx)
	if err != nil {
		return "", nil, err
	}
	ops, err = reorderIntelOperands(ctx.op, ops)
	return strings.ToUpper(ctx.op), ops, err
}

func intelSuffix(ctx opContext) string {
	for _, ptr := range ctx.ptrs {
		switch ptr {
		case "byte ptr":
			return "B"
		case "word ptr":
			return "W"
		case "dword ptr":
			return "L"
		case "qword ptr":
			return "Q"
		}
	}
	for _, arg := range ctx.args {
		if reg, ok := intelRegWidth(arg); ok {
			return reg
		}
	}
	switch ctx.op {
	case "push", "pop":
		return "Q"
	default:
		return ""
	}
}

func stripIntelPtr(arg string) (string, string) {
	lower := strings.ToLower(strings.TrimSpace(arg))
	for _, kind := range []string{"byte ptr", "word ptr", "dword ptr", "qword ptr"} {
		if strings.HasPrefix(lower, kind) {
			return strings.TrimSpace(arg[len(kind):]), kind
		}
	}
	return strings.TrimSpace(arg), ""
}

func convertIntelOperand(op, arg string) (string, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "", fmt.Errorf("empty operand")
	}
	if containsELFReloc(arg) {
		return "", fmt.Errorf("unsupported ELF relocation in operand %q", arg)
	}
	if reg, err := plan9Register(arg); err == nil {
		return reg, nil
	}
	if strings.HasPrefix(arg, "[") && strings.HasSuffix(arg, "]") {
		return convertIntelMemory(arg)
	}
	if isIntelImmediate(arg) {
		return plan9Immediate(arg), nil
	}
	return plan9Immediate(arg), nil
}

func reorderIntelOperands(op string, operands []string) ([]string, error) {
	if len(operands) == 2 && isCompareOp(op) {
		return operands, nil
	}
	if len(operands) == 2 {
		return []string{operands[1], operands[0]}, nil
	}
	lower := strings.ToLower(op)
	if len(operands) == 3 && lower == "imul" {
		return []string{operands[2], operands[1], operands[0]}, nil
	}
	if len(operands) == 3 && strings.HasPrefix(lower, "v") {
		if !isWhitelistedAVXThreeOp(op) {
			return nil, fmt.Errorf("unknown AVX three-operand instruction %q", op)
		}
		return []string{operands[1], operands[2], operands[0]}, nil
	}
	return operands, nil
}

func convertIntelMemory(arg string) (string, error) {
	inner := strings.ReplaceAll(strings.TrimSpace(arg[1:len(arg)-1]), " ", "")
	if containsRIP(inner) {
		return plan9Immediate(strings.TrimPrefix(removeRIP(inner), "+")), nil
	}
	parts := splitExpr(inner)
	var base, index, scale, disp string
	for _, part := range parts {
		if strings.Contains(part, "*") {
			pair := strings.SplitN(part, "*", 2)
			reg, err := plan9Register(pair[0])
			if err != nil {
				return "", err
			}
			index, scale = reg, pair[1]
			continue
		}
		if reg, err := plan9Register(part); err == nil {
			if base == "" {
				base = reg
			} else if index == "" {
				index, scale = reg, "1"
			} else {
				return "", fmt.Errorf("too many registers in %q", arg)
			}
			continue
		}
		disp = appendDisp(disp, part)
	}
	if base == "" {
		return plan9Immediate(disp), nil
	}
	result := disp + "(" + base + ")"
	if index != "" {
		if scale == "" {
			scale = "1"
		}
		result += "(" + index + "*" + scale + ")"
	}
	return result, nil
}

func splitExpr(s string) []string {
	if s == "" {
		return nil
	}
	var parts []string
	start := 0
	for i := 1; i < len(s); i++ {
		if s[i] == '+' || s[i] == '-' {
			parts = append(parts, strings.TrimPrefix(s[start:i], "+"))
			start = i
		}
	}
	return append(parts, strings.TrimPrefix(s[start:], "+"))
}

func appendDisp(disp, part string) string {
	if disp == "" || strings.HasPrefix(part, "-") {
		return disp + part
	}
	return disp + "+" + part
}

func containsRIP(s string) bool {
	return strings.Contains(strings.ToLower(s), "rip")
}

func removeRIP(s string) string {
	for _, candidate := range []string{"rip", "RIP", "Rip", "rIp", "riP", "RIp", "rIP", "RiP"} {
		s = strings.ReplaceAll(s, candidate, "")
	}
	return s
}
