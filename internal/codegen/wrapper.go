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
	pkg = strings.TrimSpace(pkg)
	if pkg == "" {
		pkg = "main"
	}
	arch = strings.TrimSpace(arch)
	if arch == "" {
		arch = "amd64"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\npackage %s\n\n", generatedHeader, pkg)
	if usesUnsafe(funcs) {
		b.WriteString("import \"unsafe\"\n\n")
	}
	for i, fn := range funcs {
		if i > 0 {
			b.WriteByte('\n')
		}
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
		b.WriteByte('\n')
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
	for _, fn := range funcs {
		asm = strings.ReplaceAll(asm, compilerSymbol(goos, fn.CName)+"(SB)", rawSymbol(fn.CName)+"(SB)")
	}
	asm = strings.TrimRight(asm, "\n")
	if asm != "" {
		asm += "\n\n"
	}
	for i, fn := range funcs {
		if i > 0 {
			asm += "\n"
		}
		asm += renderWrapper(fn, arch)
	}
	return asm
}

func renderWrapper(fn funcSpec, arch string) string {
	var b strings.Builder
	paramOffsets, retOffset, argSize := frameOffsets(fn)
	fmt.Fprintf(&b, "TEXT %s%s(SB), NOSPLIT, $0-%d\n", packageDot, fn.GoName, argSize)
	regs := argRegs(arch)
	reg := 0
	for i, p := range fn.Params {
		reg = renderParam(&b, p, paramOffsets[i], reg, regs, arch)
	}
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
	case "int32", "uint32":
		return "MOVW"
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
