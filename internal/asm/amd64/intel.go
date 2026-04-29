package amd64

import (
	"fmt"
	"strings"

	"github.com/frankli0324/go-c2go/internal/asm/asmutil"
)

type Intel struct {
	frame          frame
	savedRegs      uint64
	trustFixedRegs []string
}

func (*Intel) CommentPrefix() string {
	return ";"
}

func (t *Intel) TranslateInstruction(indent, line string) (string, bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return indent, false
	}
	op := strings.ToLower(fields[0])
	argsText := strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
	args := asmutil.SplitOperands(argsText)
	if reg, ok := pushPopReg(op, args); ok {
		out, ok := t.pushPop(op, reg, line)
		if ok {
			return indent + out, false
		}
	}
	if reservedMask(args)&^t.savedRegs != 0 {
		return indent + "// UNSUPPORTED: " + line, true
	}
	if op == "cdqe" {
		return indent + "MOVLQSX AX, AX", false
	}
	spec := specFor(op)
	ctx := opContext{
		op:      op,
		spec:    spec,
		ops:     args,
		convert: convertIntelOperand,
	}
	mnemonic, converted, err := intelHandlers[spec.typ](ctx)
	if err != nil {
		return indent + "// UNSUPPORTED: " + line, true
	}
	return indent + asmutil.JoinInstruction(mnemonic, converted), false
}

var intelHandlers = [...]opHandler{
	opExact:       intelExactHandler,
	opTarget:      targetHandler,
	opReturn:      returnHandler,
	opSized:       intelSizedHandler,
	opDoubleShift: intelDoubleShiftHandler,
	opCMOV:        intelCMOVHandler,
	opSETCC:       setCCHandler,
	opAVX3:        intelAVX3Handler,
}

func intelExactHandler(ctx opContext) (string, []string, error) {
	ops, err := convertIntelOperands(ctx)
	return ctx.spec.mn, ops, err
}

func intelSizedHandler(ctx opContext) (string, []string, error) {
	suffix := intelSuffix(ctx)
	if suffix == "" {
		return "", nil, fmt.Errorf("cannot infer width for %q", ctx.op)
	}
	ops, err := convertIntelOperands(ctx)
	if err != nil {
		return "", nil, err
	}
	return strings.ToUpper(ctx.op) + suffix, ops, err
}

func intelCMOVHandler(ctx opContext) (string, []string, error) {
	suffix := intelSuffix(ctx)
	if suffix == "" {
		return "", nil, fmt.Errorf("cannot infer width for %q", ctx.op)
	}
	if mnemonic, ok := cmovMnemonic(ctx.op + strings.ToLower(suffix)); ok {
		ops, err := convertIntelOperands(ctx)
		return mnemonic, ops, err
	}
	return "", nil, fmt.Errorf("unsupported cmov mnemonic %q", ctx.op)
}

func intelDoubleShiftHandler(ctx opContext) (string, []string, error) {
	if len(ctx.ops) != 3 {
		return "", nil, fmt.Errorf("%s takes three operands", ctx.op)
	}
	mnemonic := ctx.spec.mn
	if mnemonic == "" {
		suffix := intelSuffix(ctx)
		if suffix == "" || suffix == "B" {
			return "", nil, fmt.Errorf("cannot infer width for %q", ctx.op)
		}
		mnemonic = doubleShiftMnemonic(ctx.op, suffix)
	}
	ops, err := convertOperands(ctx)
	if err != nil {
		return "", nil, err
	}
	return mnemonic, []string{ops[2], ops[1], ops[0]}, nil
}

func intelAVX3Handler(ctx opContext) (string, []string, error) {
	if len(ctx.ops) != 3 {
		return "", nil, fmt.Errorf("%s takes three operands", ctx.op)
	}
	ops, err := convertOperands(ctx)
	if err != nil {
		return "", nil, err
	}
	return ctx.spec.mn, []string{ops[1], ops[2], ops[0]}, nil
}

func convertIntelOperands(ctx opContext) ([]string, error) {
	ops, err := convertOperands(ctx)
	if err != nil {
		return nil, err
	}
	return reorderIntelOperands(ctx.op, ops)
}

func intelSuffix(ctx opContext) string {
	regWidth := ""
	for _, arg := range ctx.ops {
		arg, width := stripIntelPtr(arg)
		if width != "" {
			return width
		}
		if regWidth == "" {
			regWidth, _ = intelRegWidth(arg)
		}
	}
	if regWidth != "" {
		return regWidth
	}
	return ""
}

func stripIntelPtr(arg string) (string, string) {
	lower := strings.ToLower(strings.TrimSpace(arg))
	for _, ptr := range []struct{ name, width string }{
		{"byte ptr", "B"},
		{"word ptr", "W"},
		{"dword ptr", "L"},
		{"qword ptr", "Q"},
	} {
		if strings.HasPrefix(lower, ptr.name) {
			return strings.TrimSpace(arg[len(ptr.name):]), ptr.width
		}
	}
	return strings.TrimSpace(arg), ""
}

func convertIntelOperand(op, arg string) (string, error) {
	arg, _ = stripIntelPtr(arg)
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
	return "", fmt.Errorf("unsupported operand %q", arg)
}

func reorderIntelOperands(op string, operands []string) ([]string, error) {
	if len(operands) == 2 && isCompareOp(op) {
		return operands, nil
	}
	if len(operands) == 2 {
		return []string{operands[1], operands[0]}, nil
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
			if _, err := plan9Register(pair[0]); err != nil {
				return "", err
			}
			index, scale = pair[0], pair[1]
			continue
		}
		if _, err := plan9Register(part); err == nil {
			if base == "" {
				base = part
			} else if index == "" {
				index, scale = part, "1"
			} else {
				return "", fmt.Errorf("too many registers in %q", arg)
			}
			continue
		}
		disp = appendDisp(disp, part)
	}
	if base == "" && index == "" {
		return "", fmt.Errorf("unsupported memory operand %q", arg)
	}
	return formatMemory(base, index, scale, disp)
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
