package codegen

import (
	"fmt"
	"strings"
)

const packageDot = "·"

var (
	amd64IntArgRegs = []string{"DI", "SI", "DX", "CX", "R8", "R9"}
	arm64IntArgRegs = []string{"R0", "R1", "R2", "R3", "R4", "R5", "R6", "R7"}
)

func renderDecls(pkg, arch string, funcs []funcSpec) string {
	arch = strings.TrimSpace(arch)
	if arch == "" {
		arch = "amd64"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\npackage %s\n\n", generatedHeader, packageName(pkg))
	if usesUnsafe(funcs) {
		b.WriteString("import \"unsafe\"\n\n")
	}
	for i, fn := range funcs {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintln(&b, funcSignature(fn))
	}
	return b.String()
}

func renderFallback(pkg string, funcs []funcSpec) string {
	var b strings.Builder
	fmt.Fprintf(&b, "// c2go fallback placeholder; edit implementations for unsupported architectures.\n\npackage %s\n\n", packageName(pkg))
	if usesUnsafe(funcs) {
		b.WriteString("import \"unsafe\"\n\n")
	}
	for i, fn := range funcs {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%s {\n\tpanic(%q)\n}\n", funcSignature(fn), "c2go fallback "+fn.GoName+" is not implemented")
	}
	return b.String()
}

func packageName(pkg string) string {
	pkg = strings.TrimSpace(pkg)
	if pkg == "" {
		return "main"
	}
	return pkg
}

func funcSignature(fn funcSpec) string {
	var b strings.Builder
	fmt.Fprintf(&b, "func %s(", fn.GoName)
	for j, p := range fn.Params {
		if j > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s %s", p.Name, p.Type.GoName)
	}
	b.WriteByte(')')
	if fn.Return.Kind != voidType {
		fmt.Fprintf(&b, " %s", fn.Return.GoName)
	}
	return b.String()
}

func usesUnsafe(funcs []funcSpec) bool {
	for _, fn := range funcs {
		if fn.Return.Unsafe {
			return true
		}
		for _, p := range fn.Params {
			if p.Type.Unsafe {
				return true
			}
		}
	}
	return false
}

func wrapAssembly(asm string, funcs []funcSpec, goos, arch string) string {
	specs := make(map[string]funcSpec, len(funcs))
	direct := make(map[string]bool, len(funcs))
	for _, fn := range funcs {
		csym := compilerSymbol(goos, fn.CName) + "(SB)"
		if strings.Count(asm, csym) > 1 {
			sym := rawSymbol(fn.CName) + "(SB)"
			asm = strings.ReplaceAll(asm, csym, sym)
			specs[sym] = fn
			continue
		}
		sym := packageDot + fn.GoName + "(SB)"
		asm = strings.ReplaceAll(asm, csym, sym)
		specs[sym] = fn
		direct[sym] = true
	}
	lines := strings.Split(strings.TrimRight(asm, "\n"), "\n")
	var out, wrappers []string
	for i := 0; i < len(lines); {
		sym := textSymbol(lines[i])
		fn, ok := specs[sym]
		if !ok {
			out = append(out, lines[i])
			i++
			continue
		}
		end := nextText(lines, i+1)
		if direct[sym] {
			out = append(out, inlineBlock(lines[i:end], fn, arch)...)
		} else {
			out = append(out, lines[i:end]...)
			wrappers = append(wrappers, renderWrapper(fn, arch))
		}
		i = end
	}
	if len(wrappers) > 0 {
		out = append(out, "")
		out = append(out, strings.Join(wrappers, "\n"))
	}
	return strings.Join(out, "\n") + "\n"
}

func nextText(lines []string, start int) int {
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "TEXT ") {
			return i
		}
	}
	return len(lines)
}

func inlineBlock(block []string, fn funcSpec, arch string) []string {
	paramOffsets, retOffset, argSize := frameOffsets(fn)
	var b strings.Builder
	fmt.Fprintln(&b, inlineTextHeader(block[0], fn, argSize))
	renderParams(&b, fn, arch, paramOffsets)
	out := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	for _, line := range block[1:] {
		if strings.TrimSpace(line) == "RET" && fn.Return.Kind != voidType {
			out = append(out, fmt.Sprintf("\t%s %s, ret+%d(FP)", storeOp(fn.Return, arch), returnReg(arch), retOffset))
		}
		out = append(out, line)
	}
	return out
}

func textSymbol(line string) string {
	if !strings.HasPrefix(line, "TEXT ") {
		return ""
	}
	line = line[len("TEXT "):]
	if i := strings.Index(line, ","); i >= 0 {
		return strings.TrimSpace(line[:i])
	}
	return ""
}

func inlineTextHeader(line string, fn funcSpec, argSize int) string {
	line = strings.Replace(line, "|NOFRAME", "", 1)
	if i := strings.LastIndex(line, ", "); i >= 0 {
		frame := line[i+2:]
		if j := strings.Index(frame, "-"); j >= 0 {
			frame = frame[:j]
		}
		line = line[:i+2] + frame + fmt.Sprintf("-%d", argSize)
	}
	return line
}

func renderParams(b *strings.Builder, fn funcSpec, arch string, offsets []int) {
	regs := argRegs(arch)
	reg := 0
	for i, p := range fn.Params {
		reg = renderParam(b, p, offsets[i], reg, regs, arch)
	}
}

func renderWrapper(fn funcSpec, arch string) string {
	var b strings.Builder
	paramOffsets, retOffset, argSize := frameOffsets(fn)
	fmt.Fprintf(&b, "TEXT %s%s(SB), NOSPLIT, $0-%d\n", packageDot, fn.GoName, argSize)
	renderParams(&b, fn, arch, paramOffsets)
	fmt.Fprintf(&b, "\tCALL %s(SB)\n", rawSymbol(fn.CName))
	if fn.Return.Kind != voidType {
		fmt.Fprintf(&b, "\t%s %s, ret+%d(FP)\n", storeOp(fn.Return, arch), returnReg(arch), retOffset)
	}
	b.WriteString("\tRET\n")
	return b.String()
}

func renderParam(b *strings.Builder, p paramSpec, offset int, reg int, regs []string, arch string) int {
	if p.Type.Kind == bytesType || p.Type.Kind == stringType {
		fmt.Fprintf(b, "\t%s %s+%d(FP), %s\n", ptrLoadOp(arch), p.Name, offset, regs[reg])
		fmt.Fprintf(b, "\t%s %s+%d(FP), %s\n", ptrLoadOp(arch), p.Name, offset+8, regs[reg+1])
		return reg + 2
	}
	fmt.Fprintf(b, "\t%s %s+%d(FP), %s\n", loadOp(p.Type, arch), p.Name, offset, regs[reg])
	return reg + 1
}

func argRegs(arch string) []string {
	switch strings.TrimSpace(arch) {
	case "", "amd64":
		return amd64IntArgRegs
	case "arm64":
		return arm64IntArgRegs
	default:
		return nil
	}
}

func returnReg(arch string) string {
	if strings.TrimSpace(arch) == "arm64" {
		return "R0"
	}
	return "AX"
}

func ptrLoadOp(arch string) string {
	if strings.TrimSpace(arch) == "arm64" {
		return "MOVD"
	}
	return "MOVQ"
}

func loadOp(t cType, arch string) string {
	if strings.TrimSpace(arch) != "arm64" {
		return t.Load
	}
	switch t.GoName {
	case "int8":
		return "MOVB"
	case "uint8":
		return "MOVBU"
	case "int16":
		return "MOVH"
	case "uint16":
		return "MOVHU"
	case "int32":
		return "MOVW"
	case "uint32":
		return "MOVWU"
	default:
		return "MOVD"
	}
}

func storeOp(t cType, arch string) string {
	if strings.TrimSpace(arch) != "arm64" {
		return t.Move
	}
	switch t.GoName {
	case "int8", "uint8":
		return "MOVB"
	case "int16", "uint16":
		return "MOVH"
	case "int32", "uint32":
		return "MOVW"
	default:
		return "MOVD"
	}
}

func frameOffsets(fn funcSpec) ([]int, int, int) {
	offset := 0
	offsets := make([]int, len(fn.Params))
	for i, p := range fn.Params {
		offset = align(offset, p.Type.Align)
		offsets[i] = offset
		offset += p.Type.Size
	}
	retOffset := offset
	if fn.Return.Kind != voidType {
		offset = align(offset, 8)
		retOffset = offset
		offset += fn.Return.Size
	}
	return offsets, retOffset, offset
}

func align(v, a int) int {
	if a <= 1 || v%a == 0 {
		return v
	}
	return v + a - v%a
}
