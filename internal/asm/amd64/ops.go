package amd64

import (
	"fmt"
	"strings"
)

type opSpec struct {
	typ       opType
	mn        string
	addSymbol bool
}

type opType int

const (
	opExact opType = iota
	opTarget
	opReturn
	opSized
	opDoubleShift
	opCMOV
	opSETCC
	opAVX3
)

type opHandler func(opContext) (string, []string, error)

var opSpecs = map[string]opSpec{
	"call":  {typ: opTarget, mn: "CALL", addSymbol: true},
	"callq": {typ: opTarget, mn: "CALL", addSymbol: true},
	"ret":   {typ: opReturn, mn: "RET"},
	"retq":  {typ: opReturn, mn: "RET"},

	"jmp":  {typ: opTarget, mn: "JMP", addSymbol: true},
	"jmpq": {typ: opTarget, mn: "JMP", addSymbol: true},

	"movabsq": {typ: opExact, mn: "MOVQ"},
	"movsbl":  {typ: opExact, mn: "MOVBLSX"},
	"movsbq":  {typ: opExact, mn: "MOVBQSX"},
	"movsbw":  {typ: opExact, mn: "MOVBWSX"},
	"movswl":  {typ: opExact, mn: "MOVWLSX"},
	"movswq":  {typ: opExact, mn: "MOVWQSX"},
	"movslq":  {typ: opExact, mn: "MOVLQSX"},
	"movzbl":  {typ: opExact, mn: "MOVBLZX"},
	"movzbq":  {typ: opExact, mn: "MOVBQZX"},
	"movzbw":  {typ: opExact, mn: "MOVBWZX"},
	"movzwl":  {typ: opExact, mn: "MOVWLZX"},
	"movzwq":  {typ: opExact, mn: "MOVWQZX"},
	"movzlq":  {typ: opExact, mn: "MOVLQZX"},
	"shld":    {typ: opDoubleShift},
	"shldl":   {typ: opDoubleShift, mn: "SHLL"},
	"shldq":   {typ: opDoubleShift, mn: "SHLQ"},
	"shldw":   {typ: opDoubleShift, mn: "SHLW"},
	"shrd":    {typ: opDoubleShift},
	"shrdl":   {typ: opDoubleShift, mn: "SHRL"},
	"shrdq":   {typ: opDoubleShift, mn: "SHRQ"},
	"shrdw":   {typ: opDoubleShift, mn: "SHRW"},
}

type opContext struct {
	op      string
	spec    opSpec
	ops     []string
	convert func(string, string) (string, error)
}

func specFor(op string) opSpec {
	if spec, ok := opSpecs[op]; ok {
		return spec
	}
	if strings.HasPrefix(op, "cmov") {
		return opSpec{typ: opCMOV}
	}
	if strings.HasPrefix(op, "set") {
		return opSpec{typ: opSETCC}
	}
	return opSpec{typ: opSized}
}

func returnHandler(ctx opContext) (string, []string, error) {
	if len(ctx.ops) != 0 {
		return "", nil, fmt.Errorf("return takes no operands")
	}
	return ctx.spec.mn, nil, nil
}

func targetHandler(ctx opContext) (string, []string, error) {
	ops := make([]string, len(ctx.ops))
	for i, arg := range ctx.ops {
		target, err := convertBranchTarget(arg, ctx.spec.addSymbol)
		if err != nil {
			return "", nil, err
		}
		ops[i] = target
	}
	return ctx.spec.mn, ops, nil
}

func setCCHandler(ctx opContext) (string, []string, error) {
	mnemonic, ok := setCCMnemonic(ctx.op)
	if !ok {
		return "", nil, fmt.Errorf("unsupported setcc mnemonic %q", ctx.op)
	}
	ops, err := convertOperands(ctx)
	return mnemonic, ops, err
}

func convertOperands(ctx opContext) ([]string, error) {
	out := make([]string, len(ctx.ops))
	for i, arg := range ctx.ops {
		converted, err := ctx.convert(ctx.op, arg)
		if err != nil {
			return nil, err
		}
		out[i] = converted
	}
	return out, nil
}

func doubleShiftMnemonic(op, suffix string) string {
	if strings.HasPrefix(op, "shld") {
		return "SHL" + suffix
	}
	return "SHR" + suffix
}
