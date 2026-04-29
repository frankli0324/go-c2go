package arm64

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/frankli0324/go-c2go/internal/asm/asmutil"
	"golang.org/x/arch/arm64/arm64asm"
)

var arm64Conds = map[string]arm64asm.Cond{}
var memOps = map[string]memOp{}

type memOp struct {
	load bool
}

func init() {
	opName := func(op arm64asm.Op) string {
		return strings.ToLower(op.String())
	}
	for value := uint8(0); value <= 14; value++ {
		cond := arm64asm.Cond{Value: value}
		if name := cond.String(); name != "" {
			arm64Conds[strings.ToLower(name)] = cond
		}
	}
	arm64Conds["hs"] = arm64Conds["cs"]
	arm64Conds["lo"] = arm64Conds["cc"]
	for name, cond := range arm64Conds {
		opSpecs["b."+name] = spec{mn: "B" + cond.String(), handler: condBranch}
	}
	add := func(op arm64asm.Op, h opHandler, options ...func(*spec)) {
		form := spec{op: op, mn: op.String(), handler: h}
		for _, option := range options {
			option(&form)
		}
		opSpecs[opName(op)] = form
	}
	addAll := func(h opHandler, options []func(*spec), ops ...arm64asm.Op) {
		for _, op := range ops {
			add(op, h, options...)
		}
	}
	withW := func(form *spec) { form.wmn = form.mn + "W" }
	clear := func(form *spec) { form.clearDst = true }
	rememberPage := func(form *spec) { form.rememberPage = true }
	mnemonic := func(mn string) func(*spec) {
		return func(form *spec) { form.mn = mn }
	}

	add(arm64asm.RET, instArgs(0))
	add(arm64asm.CBZ, regBranch)
	add(arm64asm.CBNZ, regBranch)
	add(arm64asm.TBZ, bitBranch)
	add(arm64asm.TBNZ, bitBranch)
	add(arm64asm.STP, pair)
	add(arm64asm.LDP, pair)
	add(arm64asm.FMOV, instArgs(2), clear)
	add(arm64asm.ADRP, adrp, mnemonic("MOVD"), rememberPage)
	add(arm64asm.ADR, adr, clear)
	addAll(rightLeftOrShifted, []func(*spec){withW, clear},
		arm64asm.ADD, arm64asm.ADDS, arm64asm.ADC, arm64asm.ADCS,
		arm64asm.AND, arm64asm.ASR, arm64asm.BIC, arm64asm.EOR,
		arm64asm.LSL, arm64asm.LSR, arm64asm.ORR, arm64asm.SUB,
		arm64asm.SUBS, arm64asm.SBC, arm64asm.SBCS, arm64asm.SDIV,
		arm64asm.UDIV,
	)
	addAll(instArgs(4), []func(*spec){withW, clear},
		arm64asm.EXTR, arm64asm.CSEL, arm64asm.BFXIL, arm64asm.UBFX, arm64asm.SBFX)
	addAll(rightLeftOrShifted, []func(*spec){clear},
		arm64asm.MUL, arm64asm.UMULL, arm64asm.UMULH,
	)
	addAll(instArgs(2), []func(*spec){clear},
		arm64asm.MVN, arm64asm.REV, arm64asm.SXTW,
	)
	addAll(instArgs(2), []func(*spec){withW, clear},
		arm64asm.NEG, arm64asm.NEGS,
	)
	add(arm64asm.ROR, instArgs(3), clear)
	add(arm64asm.MADD, instArgs(4), clear)
	add(arm64asm.MSUB, instArgs(4), clear)
	add(arm64asm.UMADDL, instArgs(4), clear)
	add(arm64asm.CSET, instArgs(2), clear)
	add(arm64asm.MOVK, moveKeep, clear)
	add(arm64asm.CMP, compare)
	add(arm64asm.CMN, compare)
	add(arm64asm.TST, compare)

	addMem := func(load bool, ops ...arm64asm.Op) {
		for _, op := range ops {
			memOps[opName(op)] = memOp{load: load}
		}
	}
	addMem(true,
		arm64asm.LDR, arm64asm.LDUR, arm64asm.LDRB, arm64asm.LDRH,
		arm64asm.LDRSB, arm64asm.LDRSH, arm64asm.LDRSW, arm64asm.LDURB,
		arm64asm.LDURH, arm64asm.LDURSB, arm64asm.LDURSH, arm64asm.LDURSW,
	)
	addMem(false,
		arm64asm.STR, arm64asm.STUR, arm64asm.STRB, arm64asm.STRH,
		arm64asm.STURB, arm64asm.STURH,
	)
}

func pair(form *spec, args []string) (string, error) {
	if len(args) != 4 {
		return "", fmt.Errorf("unsupported arm64 pair op")
	}
	mem, suffix := args[2], args[3]
	left, right, err := pairRegisters(args[0], args[1])
	if err != nil {
		return "", err
	}
	if isFloatReg(args[0]) {
		if form.mn == "LDP" {
			return "FLDPQ " + mem + ", (" + left + ", " + right + ")", nil
		}
		return "FSTPQ (" + left + ", " + right + "), " + mem, nil
	}
	if form.mn == "LDP" {
		return form.mn + suffix + " " + mem + ", (" + left + ", " + right + ")", nil
	}
	return form.mn + suffix + " (" + left + ", " + right + "), " + mem, nil
}

func pairRegisters(a, b string) (string, string, error) {
	if isFloatReg(a) {
		left, err := floatRegister(a)
		if err != nil {
			return "", "", err
		}
		right, err := floatRegister(b)
		return left, right, err
	}
	left, err := pairRegister(a)
	if err != nil {
		return "", "", err
	}
	right, err := pairRegister(b)
	return left, right, err
}

func pairRegister(name string) (string, error) {
	if reg, err := register(name); err == nil {
		return reg, nil
	}
	return pairReservedRegister(name)
}

type spec struct {
	op           arm64asm.Op
	mn           string
	wmn          string
	clearDst     bool
	rememberPage bool
	handler      opHandler
}

type opHandler func(*spec, []string) (string, error)

var opSpecs = map[string]spec{
	"b":   {mn: "JMP", handler: branch},
	"bl":  {mn: "CALL", handler: branch},
	"mov": {op: arm64asm.MOV, mn: "MOVD", wmn: "MOVW", clearDst: true, handler: instArgs(2)},
}

func translateOp(t *Translator, op string, args []string) (string, bool, error) {
	prepared := args
	if op == "add" && len(args) == 3 {
		if _, ok := pageOffsetSymbol(args[2]); ok {
			out, err := t.pageAdd(args)
			return out, true, err
		}
	}
	if mem, ok := memOps[op]; ok {
		if mem.load {
			out, err := t.load(op, args)
			return out, true, err
		}
		out, err := t.store(op, args)
		return out, true, err
	}
	if op == "stp" || op == "ldp" {
		if len(args) != 3 && len(args) != 4 {
			return "", true, fmt.Errorf("unsupported arm64 pair op")
		}
		mem, suffix, err := t.memorySuffix(args[2:])
		if err != nil {
			return "", true, err
		}
		if op == "ldp" {
			t.clear(args[0])
			t.clear(args[1])
		}
		prepared = []string{args[0], args[1], mem, suffix}
	}
	if len(args) == 2 && (strings.HasPrefix(op, "mov.") || (op == "mov" && (isVectorArg(args[0]) || isVectorArg(args[1])))) {
		t.clear(args[0])
		out, err := vectorMove(op, args)
		return out, true, err
	}
	if isVectorOp(op, args) {
		out, err := vectorOp(op, args)
		return out, true, err
	}

	form, ok := opSpecs[op]
	if !ok {
		return "", false, nil
	}
	if form.clearDst && len(args) > 0 {
		t.clear(args[0])
	}
	out, err := form.handler(&form, prepared)
	if err != nil {
		return "", true, err
	}
	if form.rememberPage {
		dst, err := operand(args[0])
		if err != nil {
			return "", true, err
		}
		t.remember(dst, pageSymbol(args[1]))
	}
	return out, true, nil
}

func (t *Translator) pageAdd(args []string) (string, error) {
	ops, err := operands(args[0], args[1])
	if err != nil {
		return "", err
	}
	sym, _ := pageOffsetSymbol(args[2])
	defer t.remember(ops[0], sym)
	if t.fullAddr[ops[1]] != sym {
		return "MOVD $" + sym + "(SB), " + ops[0], nil
	}
	if ops[0] == ops[1] {
		return "// add " + strings.Join(args, ", "), nil
	}
	return "MOVD " + ops[1] + ", " + ops[0], nil
}

func branch(form *spec, args []string) (string, error) {
	if err := needArgs(args, 1); err != nil {
		return "", err
	}
	return form.mn + " " + branchTarget(args[0]), nil
}

func condBranch(form *spec, args []string) (string, error) {
	if err := needArgs(args, 1); err != nil {
		return "", err
	}
	return form.mn + " " + args[0], nil
}

func regBranch(form *spec, args []string) (string, error) {
	if err := needArgs(args, 2); err != nil {
		return "", err
	}
	reg, err := operand(args[0])
	if err != nil {
		return "", err
	}
	return form.mn + " " + reg + ", " + branchTarget(args[1]), nil
}

func bitBranch(form *spec, args []string) (string, error) {
	if err := needArgs(args, 3); err != nil {
		return "", err
	}
	reg, err := operand(args[0])
	if err != nil {
		return "", err
	}
	return form.mn + " " + mustOperand(args[1]) + ", " + reg + ", " + branchTarget(args[2]), nil
}

func adrp(form *spec, args []string) (string, error) {
	if err := needArgs(args, 2); err != nil {
		return "", err
	}
	dst, err := operand(args[0])
	if err != nil {
		return "", err
	}
	sym := pageSymbol(args[1])
	return form.mn + " $" + sym + "(SB), " + dst, nil
}

func adr(form *spec, args []string) (string, error) {
	if err := needArgs(args, 2); err != nil {
		return "", err
	}
	dst, err := operand(args[0])
	if err != nil {
		return "", err
	}
	return form.mn + " " + args[1] + ", " + dst, nil
}

func rightLeftOrShifted(form *spec, args []string) (string, error) {
	switch len(args) {
	case 3:
		return goSyntaxForm(form, args...)
	case 4:
		return shifted(form, args)
	default:
		return "", argCountErr(args, "3 or 4")
	}
}

func shifted(form *spec, args []string) (string, error) {
	if err := needArgs(args, 4); err != nil {
		return "", err
	}
	ops, err := operands(args[0], args[1])
	if err != nil {
		return "", err
	}
	right, err := shiftedOperand(args[2], args[3])
	if err != nil {
		return "", err
	}
	return mnFor(form, args[0]) + " " + right + ", " + ops[1] + ", " + ops[0], nil
}

func moveKeep(form *spec, args []string) (string, error) {
	if err := needArgs(args, 3); err != nil {
		return "", err
	}
	ops, err := operands(args[0], args[1])
	if err != nil {
		return "", err
	}
	shift, err := shift(args[2])
	if err != nil {
		return "", err
	}
	return form.mn + " $(" + strings.TrimPrefix(ops[1], "$") + "<<" + shift + "), " + ops[0], nil
}

func compare(form *spec, args []string) (string, error) {
	if len(args) != 2 && len(args) != 3 {
		return "", argCountErr(args, "2 or 3")
	}
	if len(args) == 3 {
		left, err := shiftedOperand(args[1], args[2])
		if err != nil {
			return "", err
		}
		return form.mn + " " + left + ", " + mustOperand(args[0]), nil
	}
	return goSyntaxForm(form, args...)
}

func instArgs(want int) opHandler {
	return func(form *spec, args []string) (string, error) {
		if err := needArgs(args, want); err != nil {
			return "", err
		}
		return goSyntaxForm(form, args...)
	}
}

func goSyntaxForm(form *spec, args ...string) (string, error) {
	converted := make([]arm64asm.Arg, len(args))
	for i, arg := range args {
		out, err := asmArg(arg)
		if err != nil {
			return "", err
		}
		converted[i] = out
	}
	return goSyntax(form.op, converted...), nil
}

func goSyntax(op arm64asm.Op, args ...arm64asm.Arg) string {
	var inst arm64asm.Inst
	inst.Op = op
	for i, arg := range args {
		inst.Args[i] = arg
	}
	return arm64asm.GoSyntax(inst, 0, nil, nil)
}

func asmArg(arg string) (arm64asm.Arg, error) {
	arg = strings.TrimSpace(arg)
	if strings.HasPrefix(arg, "#") {
		value := strings.TrimPrefix(arg, "#")
		n, err := strconv.ParseInt(value, 0, 64)
		if err == nil {
			return arm64asm.Imm64{Imm: uint64(n), Decimal: true}, nil
		}
		u, err := strconv.ParseUint(value, 0, 64)
		if err != nil {
			return nil, err
		}
		return arm64asm.Imm64{Imm: u, Decimal: true}, nil
	}
	if cond, ok := arm64Conds[strings.ToLower(arg)]; ok {
		return cond, nil
	}
	if reg, err := asmRegister(arg); err == nil {
		return reg, nil
	}
	return asmVectorRegister(arg)
}

func mnFor(form *spec, dst string) string {
	if form.wmn != "" && strings.HasPrefix(strings.ToLower(strings.TrimSpace(dst)), "w") {
		return form.wmn
	}
	return form.mn
}

func shiftedOperand(regArg, shiftArg string) (string, error) {
	reg, err := operand(regArg)
	if err != nil {
		return "", err
	}
	fields := strings.Fields(strings.TrimSpace(shiftArg))
	if len(fields) != 2 {
		return "", fmt.Errorf("unsupported shift %q", shiftArg)
	}
	amount := strings.TrimPrefix(fields[1], "#")
	switch strings.ToLower(fields[0]) {
	case "lsl":
		return reg + "<<" + amount, nil
	case "lsr":
		return reg + ">>" + amount, nil
	case "asr":
		return reg + "->" + amount, nil
	case "ror":
		return reg + "@>" + amount, nil
	default:
		return "", fmt.Errorf("unsupported shift %q", shiftArg)
	}
}

func operands(args ...string) ([]string, error) {
	out := make([]string, len(args))
	for i, arg := range args {
		op, err := operand(arg)
		if err != nil {
			return nil, err
		}
		out[i] = op
	}
	return out, nil
}

func needArgs(args []string, want int) error {
	if len(args) != want {
		return argCountErr(args, fmt.Sprintf("%d", want))
	}
	return nil
}

func argCountErr(args []string, want string) error {
	return fmt.Errorf("unsupported arm64 op with %d args, want %s", len(args), want)
}

func shift(arg string) (string, error) {
	fields := strings.Fields(strings.TrimSpace(arg))
	if len(fields) != 2 || strings.ToLower(fields[0]) != "lsl" {
		return "", fmt.Errorf("unsupported arm64 shift %q", arg)
	}
	return strings.TrimPrefix(fields[1], "#"), nil
}

func branchTarget(target string) string {
	target = strings.TrimSpace(target)
	if asmutil.IsLocalLabel(target) {
		return target
	}
	return asmutil.AddSB(target)
}
