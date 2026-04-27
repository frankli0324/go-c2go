package amd64

import (
	"fmt"
	"strings"

	"github.com/frankli0324/go-c2go/internal/asm/asmutil"
)

type syntaxKind int

const (
	syntaxATT syntaxKind = iota
	syntaxIntel
)

type opSpec struct {
	typ opType
	mn  string
}

type opType int

const (
	opFixed opType = iota
	opBranch
	opSIMDExact
	opSIMDSuffix
	opATTSized
	opIntelSized
	opCMOV
	opSETCC
)

type opHandler func(opContext) (string, []string, error)

var opSpecs = map[string]opSpec{
	"call":  {typ: opBranch, mn: "CALL"},
	"callq": {typ: opBranch, mn: "CALL"},
	"ret":   {typ: opBranch, mn: "RET"},
	"retq":  {typ: opBranch, mn: "RET"},

	"jmp": {typ: opBranch, mn: "JMP"},
	"je":  {typ: opBranch, mn: "JE"},
	"jne": {typ: opBranch, mn: "JNE"},
	"jg":  {typ: opBranch, mn: "JG"},
	"jge": {typ: opBranch, mn: "JGE"},
	"jl":  {typ: opBranch, mn: "JL"},
	"jle": {typ: opBranch, mn: "JLE"},
	"ja":  {typ: opBranch, mn: "JA"},
	"jae": {typ: opBranch, mn: "JAE"},
	"jb":  {typ: opBranch, mn: "JB"},
	"jbe": {typ: opBranch, mn: "JBE"},
	"jo":  {typ: opBranch, mn: "JO"},
	"jno": {typ: opBranch, mn: "JNO"},
	"js":  {typ: opBranch, mn: "JS"},
	"jns": {typ: opBranch, mn: "JNS"},
	"jp":  {typ: opBranch, mn: "JP"},
	"jnp": {typ: opBranch, mn: "JNP"},
	"jz":  {typ: opBranch, mn: "JZ"},
	"jnz": {typ: opBranch, mn: "JNZ"},

	"movabsq":    {typ: opFixed, mn: "MOVQ"},
	"movsbl":     {typ: opFixed, mn: "MOVBLSX"},
	"movsbq":     {typ: opFixed, mn: "MOVBQSX"},
	"movsbw":     {typ: opFixed, mn: "MOVBWSX"},
	"movswl":     {typ: opFixed, mn: "MOVWLSX"},
	"movswq":     {typ: opFixed, mn: "MOVWQSX"},
	"movslq":     {typ: opFixed, mn: "MOVLQSX"},
	"movzbl":     {typ: opFixed, mn: "MOVBLZX"},
	"movzbq":     {typ: opFixed, mn: "MOVBQZX"},
	"movzbw":     {typ: opFixed, mn: "MOVBWZX"},
	"movzwl":     {typ: opFixed, mn: "MOVWLZX"},
	"movzwq":     {typ: opFixed, mn: "MOVWQZX"},
	"movzlq":     {typ: opFixed, mn: "MOVLQZX"},
	"maskmovdqu": {typ: opSIMDExact, mn: "MASKMOVOU"},
	"movdqa":     {typ: opSIMDExact, mn: "MOVO"},
	"movdqu":     {typ: opSIMDExact, mn: "MOVOU"},
	"movntdq":    {typ: opSIMDExact, mn: "MOVNTO"},

	"pshufb":       {typ: opSIMDExact, mn: "PSHUFB"},
	"pxor":         {typ: opSIMDExact, mn: "PXOR"},
	"cvtss2sd":     {typ: opSIMDExact, mn: "CVTSS2SD"},
	"cvtsd2ss":     {typ: opSIMDExact, mn: "CVTSD2SS"},
	"cvtss2si":     {typ: opSIMDExact, mn: "CVTSS2SI"},
	"cvtsd2si":     {typ: opSIMDExact, mn: "CVTSD2SI"},
	"cvttss2si":    {typ: opSIMDExact, mn: "CVTTSS2SI"},
	"cvttsd2si":    {typ: opSIMDExact, mn: "CVTTSD2SI"},
	"vextracti128": {typ: opSIMDExact, mn: "VEXTRACTI128"},
	"vinserti128":  {typ: opSIMDExact, mn: "VINSERTI128"},
	"vpaddb":       {typ: opSIMDExact, mn: "VPADDB"},
	"vpaddd":       {typ: opSIMDExact, mn: "VPADDD"},
	"vpaddq":       {typ: opSIMDExact, mn: "VPADDQ"},
	"vpaddw":       {typ: opSIMDExact, mn: "VPADDW"},
	"vpand":        {typ: opSIMDExact, mn: "VPAND"},
	"vpandn":       {typ: opSIMDExact, mn: "VPANDN"},
	"vpavgb":       {typ: opSIMDExact, mn: "VPAVGB"},
	"vpavgw":       {typ: opSIMDExact, mn: "VPAVGW"},
	"vpbroadcastq": {typ: opSIMDExact, mn: "VPBROADCASTQ"},
	"vpcmpeqb":     {typ: opSIMDExact, mn: "VPCMPEQB"},
	"vpcmpeqd":     {typ: opSIMDExact, mn: "VPCMPEQD"},
	"vpcmpeqq":     {typ: opSIMDExact, mn: "VPCMPEQQ"},
	"vpcmpeqw":     {typ: opSIMDExact, mn: "VPCMPEQW"},
	"vpcmpgtb":     {typ: opSIMDExact, mn: "VPCMPGTB"},
	"vpcmpgtd":     {typ: opSIMDExact, mn: "VPCMPGTD"},
	"vpcmpgtq":     {typ: opSIMDExact, mn: "VPCMPGTQ"},
	"vpcmpgtw":     {typ: opSIMDExact, mn: "VPCMPGTW"},
	"vpermq":       {typ: opSIMDExact, mn: "VPERMQ"},
	"vpextrq":      {typ: opSIMDExact, mn: "VPEXTRQ"},
	"vpmaxsb":      {typ: opSIMDExact, mn: "VPMAXSB"},
	"vpmaxsd":      {typ: opSIMDExact, mn: "VPMAXSD"},
	"vpmaxsw":      {typ: opSIMDExact, mn: "VPMAXSW"},
	"vpmaxub":      {typ: opSIMDExact, mn: "VPMAXUB"},
	"vpmaxud":      {typ: opSIMDExact, mn: "VPMAXUD"},
	"vpmaxuw":      {typ: opSIMDExact, mn: "VPMAXUW"},
	"vpminsb":      {typ: opSIMDExact, mn: "VPMINSB"},
	"vpminsd":      {typ: opSIMDExact, mn: "VPMINSD"},
	"vpminsw":      {typ: opSIMDExact, mn: "VPMINSW"},
	"vpminub":      {typ: opSIMDExact, mn: "VPMINUB"},
	"vpminud":      {typ: opSIMDExact, mn: "VPMINUD"},
	"vpminuw":      {typ: opSIMDExact, mn: "VPMINUW"},
	"vpmulld":      {typ: opSIMDExact, mn: "VPMULLD"},
	"vpmullw":      {typ: opSIMDExact, mn: "VPMULLW"},
	"vpmuludq":     {typ: opSIMDExact, mn: "VPMULUDQ"},
	"vpor":         {typ: opSIMDExact, mn: "VPOR"},
	"vpshufb":      {typ: opSIMDExact, mn: "VPSHUFB"},
	"vpshufd":      {typ: opSIMDExact, mn: "VPSHUFD"},
	"vpsllw":       {typ: opSIMDExact, mn: "VPSLLW"},
	"vpslld":       {typ: opSIMDExact, mn: "VPSLLD"},
	"vpsllq":       {typ: opSIMDExact, mn: "VPSLLQ"},
	"vpsraw":       {typ: opSIMDExact, mn: "VPSRAW"},
	"vpsrad":       {typ: opSIMDExact, mn: "VPSRAD"},
	"vpsrlw":       {typ: opSIMDExact, mn: "VPSRLW"},
	"vpsrld":       {typ: opSIMDExact, mn: "VPSRLD"},
	"vpsrlq":       {typ: opSIMDExact, mn: "VPSRLQ"},
	"vpsubb":       {typ: opSIMDExact, mn: "VPSUBB"},
	"vpsubd":       {typ: opSIMDExact, mn: "VPSUBD"},
	"vpsubq":       {typ: opSIMDExact, mn: "VPSUBQ"},
	"vpsubw":       {typ: opSIMDExact, mn: "VPSUBW"},
	"vpxor":        {typ: opSIMDExact, mn: "VPXOR"},
	"vzeroupper":   {typ: opSIMDExact, mn: "VZEROUPPER"},
}

type opContext struct {
	syntax syntaxKind
	op     string
	args   []string
	ptrs   []string
	spec   opSpec
	ops    []string
}

func translateAMD64(op string, args []string, syntax syntaxKind, handlers map[opType]opHandler) (string, error) {
	op = strings.ToLower(op)
	ptrs := make([]string, len(args))
	clean := make([]string, len(args))
	for i, arg := range args {
		operand, ptr := cleanOperand(syntax, arg)
		ptrs[i] = ptr
		clean[i] = operand
	}
	spec := specFor(syntax, op)
	mnemonic, converted, err := handlers[spec.typ](opContext{
		syntax: syntax,
		op:     op,
		args:   args,
		ptrs:   ptrs,
		spec:   spec,
		ops:    clean,
	})
	if err != nil {
		return "", err
	}
	converted, err = reorderOperands(syntax, op, converted)
	if err != nil {
		return "", err
	}
	return asmutil.JoinInstruction(mnemonic, converted), nil
}

func specFor(syntax syntaxKind, op string) opSpec {
	if spec, ok := opSpecs[op]; ok {
		return spec
	}
	if isSIMDSuffixMnemonic(op) {
		return opSpec{typ: opSIMDSuffix}
	}
	if strings.HasPrefix(op, "cmov") {
		return opSpec{typ: opCMOV}
	}
	if strings.HasPrefix(op, "set") {
		return opSpec{typ: opSETCC}
	}
	if syntax == syntaxIntel {
		return opSpec{typ: opIntelSized}
	}
	return opSpec{typ: opATTSized}
}

func fixedHandler(ctx opContext) (string, []string, error) {
	ops, err := convertOperands(ctx)
	return ctx.spec.mn, ops, err
}

func branchHandler(ctx opContext) (string, []string, error) {
	ops := make([]string, len(ctx.ops))
	for i, arg := range ctx.ops {
		ops[i] = convertBranchTarget(ctx.op, arg)
	}
	return ctx.spec.mn, ops, nil
}

func simdExactHandler(ctx opContext) (string, []string, error) {
	ops, err := convertSIMDOperands(ctx)
	return ctx.spec.mn, ops, err
}

func simdSuffixHandler(ctx opContext) (string, []string, error) {
	ops, err := convertSIMDOperands(ctx)
	return strings.ToUpper(ctx.op), ops, err
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
		converted, err := convertOperand(ctx.syntax, ctx.op, arg)
		if err != nil {
			return nil, err
		}
		out[i] = converted
	}
	return out, nil
}

func convertSIMDOperands(ctx opContext) ([]string, error) {
	out, err := convertOperands(ctx)
	if err != nil || ctx.syntax != syntaxATT {
		return out, err
	}
	for i, arg := range ctx.ops {
		if strings.Contains(strings.ToLower(arg), "(%rip)") {
			out[i] = strings.TrimPrefix(out[i], "$")
		}
	}
	return out, nil
}

func cleanOperand(syntax syntaxKind, arg string) (string, string) {
	if syntax == syntaxIntel {
		return stripIntelPtr(arg)
	}
	return arg, ""
}

func convertOperand(syntax syntaxKind, op, arg string) (string, error) {
	if syntax == syntaxIntel {
		return convertIntelOperand(op, arg)
	}
	return convertATTOperand(op, arg)
}

func reorderOperands(syntax syntaxKind, op string, operands []string) ([]string, error) {
	if syntax == syntaxIntel {
		return reorderIntelOperands(op, operands)
	}
	return reorderATTOperands(op, operands), nil
}
