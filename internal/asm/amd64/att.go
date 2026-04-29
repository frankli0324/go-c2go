package amd64

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/frankli0324/go-c2go/internal/asm/asmutil"
)

var attMemoryRE = regexp.MustCompile(`^(.*)\(%?([^,]*)(?:,%([^,]*)(?:,(\d+))?)?\)$`)

type ATT struct {
	frame          frame
	savedRegs      uint64
	trustFixedRegs []string
}

func (*ATT) CommentPrefix() string {
	return "#"
}

func (t *ATT) TranslateInstruction(indent, line string) (string, bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return indent, false
	}
	rawOp := fields[0]
	op := strings.ToLower(rawOp)
	argsText := strings.TrimSpace(strings.TrimPrefix(line, rawOp))
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
	if op == "cltq" {
		return indent + "MOVLQSX AX, AX", false
	}
	spec := specFor(op)
	mnemonic, converted, err := attHandlers[spec.typ](opContext{
		op:      op,
		spec:    spec,
		ops:     args,
		convert: convertATTOperand,
	})
	if err != nil {
		return indent + "// UNSUPPORTED: " + line, true
	}
	return indent + asmutil.JoinInstruction(mnemonic, converted), false
}

var attHandlers = [...]opHandler{
	opExact:       attExactHandler,
	opTarget:      targetHandler,
	opReturn:      returnHandler,
	opSized:       attSizedHandler,
	opDoubleShift: attDoubleShiftHandler,
	opCMOV:        attCMOVHandler,
	opSETCC:       setCCHandler,
	opAVX3:        attExactHandler,
}

func attExactHandler(ctx opContext) (string, []string, error) {
	ops, err := convertOperands(ctx)
	if err != nil {
		return "", nil, err
	}
	for i, arg := range ctx.ops {
		if converted, ok, err := convertATTMemoryOperand(strings.TrimSpace(arg)); err != nil {
			return "", nil, err
		} else if ok && converted == ops[i] {
			ops[i] = strings.TrimPrefix(ops[i], "$")
		}
	}
	return ctx.spec.mn, reorderATTOperands(ctx.op, ops), nil
}

func attSizedHandler(ctx opContext) (string, []string, error) {
	if len(ctx.op) > 1 && strings.ContainsRune("bwlq", rune(ctx.op[len(ctx.op)-1])) {
		ops, err := convertOperands(ctx)
		return strings.ToUpper(ctx.op), reorderATTOperands(ctx.op, ops), err
	}
	return "", nil, fmt.Errorf("unsupported mnemonic %q", ctx.op)
}

func attCMOVHandler(ctx opContext) (string, []string, error) {
	if mnemonic, ok := cmovMnemonic(ctx.op); ok {
		ops, err := convertOperands(ctx)
		return mnemonic, reorderATTOperands(ctx.op, ops), err
	}
	return "", nil, fmt.Errorf("unsupported cmov mnemonic %q", ctx.op)
}

func attDoubleShiftHandler(ctx opContext) (string, []string, error) {
	if len(ctx.ops) != 3 {
		return "", nil, fmt.Errorf("%s takes three operands", ctx.op)
	}
	mnemonic := ctx.spec.mn
	if mnemonic == "" {
		return "", nil, fmt.Errorf("unsupported mnemonic %q", ctx.op)
	}
	ops, err := convertOperands(ctx)
	return mnemonic, ops, err
}

func convertATTOperand(op, arg string) (string, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "", fmt.Errorf("empty operand")
	}
	if containsELFReloc(arg) {
		return "", fmt.Errorf("unsupported ELF relocation in operand %q", arg)
	}
	if strings.HasPrefix(arg, "%") {
		return plan9Register(strings.TrimPrefix(arg, "%"))
	}
	if strings.HasPrefix(arg, "$") {
		value := strings.TrimPrefix(arg, "$")
		return plan9Immediate(value), nil
	}
	if converted, ok, err := convertATTMemoryOperand(arg); ok || err != nil {
		if err != nil {
			return "", err
		}
		if strings.HasPrefix(op, "lea") && strings.HasPrefix(converted, "$") {
			converted = strings.TrimPrefix(converted, "$")
		}
		return converted, nil
	}
	return "", fmt.Errorf("unsupported operand %q", arg)
}

func convertATTMemoryOperand(arg string) (string, bool, error) {
	if !strings.Contains(arg, "(") || !strings.HasSuffix(arg, ")") {
		return "", false, nil
	}
	converted, err := convertATTMemory(arg)
	return converted, true, err
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
	if base == "" && index == "" {
		return plan9Immediate(disp), nil
	}
	return formatMemory(base, index, scale, disp)
}

func reorderATTOperands(op string, operands []string) []string {
	if len(operands) == 1 && isUnaryOneShift(op) {
		return []string{"$1", operands[0]}
	}
	if len(operands) == 2 && isCompareOp(op) {
		return []string{operands[1], operands[0]}
	}
	return operands
}

func isCompareOp(op string) bool {
	op = strings.ToLower(op)
	if len(op) > 1 && strings.ContainsRune("bwlq", rune(op[len(op)-1])) {
		op = op[:len(op)-1]
	}
	return op == "cmp"
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
