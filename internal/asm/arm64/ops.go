package arm64

import (
	"fmt"
	"strings"

	"github.com/frankli0324/go-c2go/internal/asm/asmutil"
)

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
	left, leftErr := register(a)
	right, rightErr := register(b)
	if leftErr == nil && rightErr == nil {
		return left, right, nil
	}
	if leftErr != nil {
		left, leftErr = pairReservedRegister(a)
	}
	if rightErr != nil {
		right, rightErr = pairReservedRegister(b)
	}
	if leftErr != nil {
		return "", "", leftErr
	}
	if rightErr != nil {
		return "", "", rightErr
	}
	return left, right, nil
}

func floatMove(form *spec, args []string) (string, error) {
	if len(args) != 2 {
		return "", fmt.Errorf("unsupported arm64 float move")
	}
	dst, src := strings.TrimSpace(args[0]), strings.TrimSpace(args[1])
	if !strings.HasPrefix(strings.ToLower(dst), "x") && !strings.HasPrefix(strings.ToLower(dst), "w") {
		return "", fmt.Errorf("unsupported arm64 float move")
	}
	out, err := operand(dst)
	if err != nil {
		return "", err
	}
	in, err := floatRegister(src)
	if err != nil {
		return "", err
	}
	return "FMOVD " + in + ", " + out, nil
}

type spec struct {
	typ          opType
	mn           string
	wmn          string
	clearDst     bool
	rememberPage bool
}

type opType int

const (
	opLiteral opType = iota
	opBranch
	opCondBranch
	opRegBranch
	opBitBranch
	opPair
	opFloatMove
	opADRP
	opADR
	opRightLeft
	opSrcDst
	opRotate
	opExtract
	opMulAdd
	opCondSelect
	opCondSet
	opBitfield
	opMove
	opMoveKeep
	opCompare
)

type opHandler func(*spec, []string) (string, error)

var opSpecs = map[string]spec{
	"ret":  {typ: opLiteral, mn: "RET"},
	"b":    {typ: opBranch, mn: "JMP"},
	"bl":   {typ: opBranch, mn: "CALL"},
	"b.eq": {typ: opCondBranch, mn: "BEQ"},
	"b.ne": {typ: opCondBranch, mn: "BNE"},
	"b.lt": {typ: opCondBranch, mn: "BLT"},
	"b.le": {typ: opCondBranch, mn: "BLE"},
	"b.gt": {typ: opCondBranch, mn: "BGT"},
	"b.ge": {typ: opCondBranch, mn: "BGE"},
	"b.lo": {typ: opCondBranch, mn: "BLO"},
	"b.ls": {typ: opCondBranch, mn: "BLS"},
	"b.hi": {typ: opCondBranch, mn: "BHI"},
	"b.hs": {typ: opCondBranch, mn: "BHS"},
	"b.mi": {typ: opCondBranch, mn: "BMI"},
	"b.pl": {typ: opCondBranch, mn: "BPL"},
	"cbz":  {typ: opRegBranch, mn: "CBZ"},
	"cbnz": {typ: opRegBranch, mn: "CBNZ"},
	"tbz":  {typ: opBitBranch, mn: "TBZ"},
	"tbnz": {typ: opBitBranch, mn: "TBNZ"},

	"stp":  {typ: opPair, mn: "STP"},
	"ldp":  {typ: opPair, mn: "LDP"},
	"fmov": {typ: opFloatMove, mn: "FMOVD", clearDst: true},
	"adrp": {typ: opADRP, mn: "MOVD", rememberPage: true},
	"adr":  {typ: opADR, mn: "ADR", clearDst: true},

	"add":    {typ: opRightLeft, mn: "ADD", wmn: "ADDW", clearDst: true},
	"adds":   {typ: opRightLeft, mn: "ADDS", wmn: "ADDSW", clearDst: true},
	"adc":    {typ: opRightLeft, mn: "ADC", wmn: "ADCW", clearDst: true},
	"adcs":   {typ: opRightLeft, mn: "ADCS", wmn: "ADCSW", clearDst: true},
	"and":    {typ: opRightLeft, mn: "AND", wmn: "ANDW", clearDst: true},
	"asr":    {typ: opRightLeft, mn: "ASR", wmn: "ASRW", clearDst: true},
	"bic":    {typ: opRightLeft, mn: "BIC", wmn: "BICW", clearDst: true},
	"eor":    {typ: opRightLeft, mn: "EOR", wmn: "EORW", clearDst: true},
	"lsl":    {typ: opRightLeft, mn: "LSL", wmn: "LSLW", clearDst: true},
	"lsr":    {typ: opRightLeft, mn: "LSR", wmn: "LSRW", clearDst: true},
	"orr":    {typ: opRightLeft, mn: "ORR", wmn: "ORRW", clearDst: true},
	"sub":    {typ: opRightLeft, mn: "SUB", wmn: "SUBW", clearDst: true},
	"subs":   {typ: opRightLeft, mn: "SUBS", wmn: "SUBSW", clearDst: true},
	"sbc":    {typ: opRightLeft, mn: "SBC", wmn: "SBCW", clearDst: true},
	"sbcs":   {typ: opRightLeft, mn: "SBCS", wmn: "SBCSW", clearDst: true},
	"mul":    {typ: opRightLeft, mn: "MUL", clearDst: true},
	"sdiv":   {typ: opRightLeft, mn: "SDIV", wmn: "SDIVW", clearDst: true},
	"udiv":   {typ: opRightLeft, mn: "UDIV", wmn: "UDIVW", clearDst: true},
	"mvn":    {typ: opSrcDst, mn: "MVN", clearDst: true},
	"neg":    {typ: opSrcDst, mn: "NEG", wmn: "NEGW", clearDst: true},
	"negs":   {typ: opSrcDst, mn: "NEGS", wmn: "NEGSW", clearDst: true},
	"rev":    {typ: opSrcDst, mn: "REV", clearDst: true},
	"sxtw":   {typ: opSrcDst, mn: "SXTW", clearDst: true},
	"ror":    {typ: opRotate, mn: "ROR", clearDst: true},
	"extr":   {typ: opExtract, mn: "EXTR", wmn: "EXTRW", clearDst: true},
	"umull":  {typ: opRightLeft, mn: "UMULL", clearDst: true},
	"umulh":  {typ: opRightLeft, mn: "UMULH", clearDst: true},
	"madd":   {typ: opMulAdd, mn: "MADD", clearDst: true},
	"msub":   {typ: opMulAdd, mn: "MSUB", clearDst: true},
	"umaddl": {typ: opMulAdd, mn: "UMADDL", clearDst: true},
	"csel":   {typ: opCondSelect, mn: "CSEL", wmn: "CSELW", clearDst: true},
	"cset":   {typ: opCondSet, mn: "CSET", clearDst: true},
	"bfxil":  {typ: opBitfield, mn: "BFXIL", wmn: "BFXILW", clearDst: true},
	"ubfx":   {typ: opBitfield, mn: "UBFX", wmn: "UBFXW", clearDst: true},
	"sbfx":   {typ: opBitfield, mn: "SBFX", wmn: "SBFXW", clearDst: true},
	"mov":    {typ: opMove, mn: "MOVD", wmn: "MOVW", clearDst: true},
	"movk":   {typ: opMoveKeep, mn: "MOVK", clearDst: true},
	"cmp":    {typ: opCompare, mn: "CMP"},
	"cmn":    {typ: opCompare, mn: "CMN"},
	"tst":    {typ: opCompare, mn: "TST"},
}

var opHandlers = map[opType]opHandler{
	opLiteral:    literal,
	opBranch:     branch,
	opCondBranch: condBranch,
	opRegBranch:  regBranch,
	opBitBranch:  bitBranch,
	opPair:       pair,
	opFloatMove:  floatMove,
	opADRP:       adrp,
	opADR:        adr,
	opRightLeft:  rightLeftOrShifted,
	opSrcDst:     srcDst,
	opRotate:     rotate,
	opExtract:    extract,
	opMulAdd:     mulAdd,
	opCondSelect: condSelect,
	opCondSet:    condSet,
	opBitfield:   bitfield,
	opMove:       move,
	opMoveKeep:   moveKeep,
	opCompare:    compare,
}

func translateOp(t *Translator, op string, args []string) (string, bool, error) {
	prepared := args
	if op == "add" && len(args) == 3 {
		if _, ok := pageOffsetSymbol(args[2]); ok {
			out, err := t.pageAdd(args)
			return out, true, err
		}
	}
	if strings.HasPrefix(op, "ldr") || strings.HasPrefix(op, "ldur") {
		out, err := t.load(op, args)
		return out, true, err
	}
	if strings.HasPrefix(op, "str") || strings.HasPrefix(op, "stur") {
		out, err := t.store(op, args)
		return out, true, err
	}
	if op == "stp" || op == "ldp" {
		if len(args) != 3 && len(args) != 4 {
			return "", true, fmt.Errorf("unsupported arm64 pair op")
		}
		mem, suffix, err := t.pairMemory(args[2:])
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
	out, err := opHandlers[form.typ](&form, prepared)
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

func literal(form *spec, args []string) (string, error) {
	if err := needArgs(args, 0); err != nil {
		return "", err
	}
	return form.mn, nil
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

func srcDst(form *spec, args []string) (string, error) {
	if err := needArgs(args, 2); err != nil {
		return "", err
	}
	ops, err := operands(args[0], args[1])
	if err != nil {
		return "", err
	}
	return mnFor(form, args[0]) + " " + ops[1] + ", " + ops[0], nil
}

func rotate(form *spec, args []string) (string, error) {
	if err := needArgs(args, 3); err != nil {
		return "", err
	}
	ops, err := operands(args[0], args[1])
	if err != nil {
		return "", err
	}
	return form.mn + " " + mustOperand(args[2]) + ", " + ops[1] + ", " + ops[0], nil
}

func extract(form *spec, args []string) (string, error) {
	if err := needArgs(args, 4); err != nil {
		return "", err
	}
	ops, err := operands(args[0], args[1], args[2])
	if err != nil {
		return "", err
	}
	return mnFor(form, args[0]) + " " + mustOperand(args[3]) + ", " + ops[2] + ", " + ops[1] + ", " + ops[0], nil
}

func rightLeftDst(form *spec, args []string) (string, error) {
	if err := needArgs(args, 3); err != nil {
		return "", err
	}
	ops, err := operands(args[0], args[1], args[2])
	if err != nil {
		return "", err
	}
	return mnFor(form, args[0]) + " " + ops[2] + ", " + ops[1] + ", " + ops[0], nil
}

func rightLeftOrShifted(form *spec, args []string) (string, error) {
	switch len(args) {
	case 3:
		return rightLeftDst(form, args)
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

func mulAdd(form *spec, args []string) (string, error) {
	if err := needArgs(args, 4); err != nil {
		return "", err
	}
	ops, err := operands(args[0], args[1], args[2], args[3])
	if err != nil {
		return "", err
	}
	return form.mn + " " + ops[2] + ", " + ops[3] + ", " + ops[1] + ", " + ops[0], nil
}

func condSelect(form *spec, args []string) (string, error) {
	if err := needArgs(args, 4); err != nil {
		return "", err
	}
	ops, err := operands(args[0], args[1], args[2])
	if err != nil {
		return "", err
	}
	return mnFor(form, args[0]) + " " + strings.ToUpper(args[3]) + ", " + ops[1] + ", " + ops[2] + ", " + ops[0], nil
}

func condSet(form *spec, args []string) (string, error) {
	if err := needArgs(args, 2); err != nil {
		return "", err
	}
	dst, err := operand(args[0])
	if err != nil {
		return "", err
	}
	return form.mn + " " + strings.ToUpper(args[1]) + ", " + dst, nil
}

func bitfield(form *spec, args []string) (string, error) {
	if err := needArgs(args, 4); err != nil {
		return "", err
	}
	ops, err := operands(args[0], args[1])
	if err != nil {
		return "", err
	}
	return mnFor(form, args[0]) + " " + mustOperand(args[2]) + ", " + ops[1] + ", " + mustOperand(args[3]) + ", " + ops[0], nil
}

func move(form *spec, args []string) (string, error) {
	if err := needArgs(args, 2); err != nil {
		return "", err
	}
	ops, err := operands(args[0], args[1])
	if err != nil {
		return "", err
	}
	return mnFor(form, args[0]) + " " + ops[1] + ", " + ops[0], nil
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
	if err := needArgs(args, 2); err != nil {
		return "", err
	}
	return form.mn + " " + mustOperand(args[1]) + ", " + mustOperand(args[0]), nil
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
