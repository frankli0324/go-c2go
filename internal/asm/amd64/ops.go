package amd64

import (
	"fmt"
	"strings"
)

type opSpec struct {
	typ opType
	mn  string
}

type opType int

const (
	opFixed opType = iota
	opCall
	opJump
	opCondBranch
	opCondBranchSuffix
	opReturn
	opSIMDExact
	opSIMDSuffix
	opSized
	opDoubleShift
	opCMOV
	opSETCC
)

type opHandler func(opContext) (string, []string, error)

var opSpecs = map[string]opSpec{
	"call":  {typ: opCall, mn: "CALL"},
	"callq": {typ: opCall, mn: "CALL"},
	"ret":   {typ: opReturn, mn: "RET"},
	"retq":  {typ: opReturn, mn: "RET"},

	"jmp":  {typ: opJump, mn: "JMP"},
	"jmpq": {typ: opJump, mn: "JMP"},
	"je":   {typ: opCondBranch, mn: "JE"},
	"jne":  {typ: opCondBranch, mn: "JNE"},
	"jg":   {typ: opCondBranch, mn: "JG"},
	"jge":  {typ: opCondBranch, mn: "JGE"},
	"jl":   {typ: opCondBranch, mn: "JL"},
	"jle":  {typ: opCondBranch, mn: "JLE"},
	"ja":   {typ: opCondBranch, mn: "JA"},
	"jae":  {typ: opCondBranch, mn: "JAE"},
	"jb":   {typ: opCondBranch, mn: "JB"},
	"jbe":  {typ: opCondBranch, mn: "JBE"},
	"jo":   {typ: opCondBranch, mn: "JO"},
	"jno":  {typ: opCondBranch, mn: "JNO"},
	"js":   {typ: opCondBranch, mn: "JS"},
	"jns":  {typ: opCondBranch, mn: "JNS"},
	"jp":   {typ: opCondBranch, mn: "JP"},
	"jnp":  {typ: opCondBranch, mn: "JNP"},
	"jz":   {typ: opCondBranch, mn: "JZ"},
	"jnz":  {typ: opCondBranch, mn: "JNZ"},

	"jeq":  {typ: opCondBranchSuffix, mn: "JE"},
	"jneq": {typ: opCondBranchSuffix, mn: "JNE"},
	"jgq":  {typ: opCondBranchSuffix, mn: "JG"},
	"jgeq": {typ: opCondBranchSuffix, mn: "JGE"},
	"jlq":  {typ: opCondBranchSuffix, mn: "JL"},
	"jleq": {typ: opCondBranchSuffix, mn: "JLE"},
	"jaq":  {typ: opCondBranchSuffix, mn: "JA"},
	"jaeq": {typ: opCondBranchSuffix, mn: "JAE"},
	"jbq":  {typ: opCondBranchSuffix, mn: "JB"},
	"jbeq": {typ: opCondBranchSuffix, mn: "JBE"},
	"joq":  {typ: opCondBranchSuffix, mn: "JO"},
	"jnoq": {typ: opCondBranchSuffix, mn: "JNO"},
	"jsq":  {typ: opCondBranchSuffix, mn: "JS"},
	"jnsq": {typ: opCondBranchSuffix, mn: "JNS"},
	"jpq":  {typ: opCondBranchSuffix, mn: "JP"},
	"jnpq": {typ: opCondBranchSuffix, mn: "JNP"},
	"jzq":  {typ: opCondBranchSuffix, mn: "JZ"},
	"jnzq": {typ: opCondBranchSuffix, mn: "JNZ"},

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
	"shld":       {typ: opDoubleShift},
	"shldl":      {typ: opDoubleShift, mn: "SHLL"},
	"shldq":      {typ: opDoubleShift, mn: "SHLQ"},
	"shldw":      {typ: opDoubleShift, mn: "SHLW"},
	"shrd":       {typ: opDoubleShift},
	"shrdl":      {typ: opDoubleShift, mn: "SHRL"},
	"shrdq":      {typ: opDoubleShift, mn: "SHRQ"},
	"shrdw":      {typ: opDoubleShift, mn: "SHRW"},

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
	op      string
	args    []string
	ptrs    []string
	spec    opSpec
	ops     []string
	convert func(string, string) (string, error)
}

func specFor(op string) opSpec {
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
	return opSpec{typ: opSized}
}

func returnHandler(ctx opContext) (string, []string, error) {
	if len(ctx.ops) != 0 {
		return "", nil, fmt.Errorf("return takes no operands")
	}
	return ctx.spec.mn, nil, nil
}

func targetBranchHandler(ctx opContext) (string, []string, error) {
	addSymbol := ctx.spec.typ == opCall || ctx.spec.typ == opJump
	ops := make([]string, len(ctx.ops))
	for i, arg := range ctx.ops {
		target, err := convertBranchTarget(arg, addSymbol)
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

func convertSIMDOperands(ctx opContext) ([]string, error) {
	return convertOperands(ctx)
}

func doubleShiftMnemonic(op, suffix string) string {
	if strings.HasPrefix(op, "shld") {
		return "SHL" + suffix
	}
	return "SHR" + suffix
}
