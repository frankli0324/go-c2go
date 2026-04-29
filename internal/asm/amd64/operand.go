package amd64

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/frankli0324/go-c2go/internal/asm/asmutil"
	"golang.org/x/arch/x86/x86asm"
)

type archRegister struct {
	plan9  string
	width  string
	number int
}

var amd64Registers = map[string]archRegister{}

func init() {
	add := func(reg x86asm.Reg, plan9, width string, number int, names ...string) {
		info := archRegister{plan9: plan9, width: width, number: number}
		if name := reg.String(); name != "" {
			amd64Registers[strings.ToLower(name)] = info
		}
		for _, name := range names {
			amd64Registers[strings.ToLower(name)] = info
		}
	}
	addName := func(plan9 string, names ...string) {
		info := archRegister{plan9: plan9, number: -1}
		for _, name := range names {
			amd64Registers[name] = info
		}
	}
	add(x86asm.AL, "AL", "B", 0)
	add(x86asm.BL, "BL", "B", 3)
	add(x86asm.CL, "CL", "B", 1)
	add(x86asm.DL, "DL", "B", 2)
	add(x86asm.AH, "AH", "B", 0)
	add(x86asm.BH, "BH", "B", 3)
	add(x86asm.CH, "CH", "B", 1)
	add(x86asm.DH, "DH", "B", 2)
	add(x86asm.SPB, "SPB", "B", 4, "spl")
	add(x86asm.BPB, "BPB", "B", 5, "bpl")
	add(x86asm.SIB, "SIB", "B", 6, "sil")
	add(x86asm.DIB, "DIB", "B", 7, "dil")
	add(x86asm.RAX, "AX", "Q", 0)
	add(x86asm.RBX, "BX", "Q", 3)
	add(x86asm.RCX, "CX", "Q", 1)
	add(x86asm.RDX, "DX", "Q", 2)
	add(x86asm.RSI, "SI", "Q", 6)
	add(x86asm.RDI, "DI", "Q", 7)
	add(x86asm.RBP, "BP", "Q", 5)
	add(x86asm.RSP, "SP", "Q", 4)
	add(x86asm.EAX, "AX", "L", 0)
	add(x86asm.EBX, "BX", "L", 3)
	add(x86asm.ECX, "CX", "L", 1)
	add(x86asm.EDX, "DX", "L", 2)
	add(x86asm.ESI, "SI", "L", 6)
	add(x86asm.EDI, "DI", "L", 7)
	add(x86asm.EBP, "BP", "L", 5)
	add(x86asm.ESP, "SP", "L", 4)
	add(x86asm.AX, "AX", "W", 0)
	add(x86asm.BX, "BX", "W", 3)
	add(x86asm.CX, "CX", "W", 1)
	add(x86asm.DX, "DX", "W", 2)
	add(x86asm.SI, "SI", "W", 6)
	add(x86asm.DI, "DI", "W", 7)
	add(x86asm.BP, "BP", "W", 5)
	add(x86asm.SP, "SP", "W", 4)
	add(x86asm.RIP, "PC", "", -1)
	for i := 8; i <= 15; i++ {
		n := strconv.Itoa(i)
		add(x86asm.R8+x86asm.Reg(i-8), "R"+n, "Q", i)
		add(x86asm.R8L+x86asm.Reg(i-8), "R"+n, "L", i, "r"+n+"d")
		add(x86asm.R8W+x86asm.Reg(i-8), "R"+n, "W", i, "r"+n+"w")
		add(x86asm.R8B+x86asm.Reg(i-8), "R"+n+"B", "B", i)
	}
	for i := 0; i <= 31; i++ {
		n := strconv.Itoa(i)
		if i <= 15 {
			add(x86asm.X0+x86asm.Reg(i), "X"+n, "", -1, "xmm"+n)
		} else {
			addName("X"+n, "xmm"+n)
		}
		addName("Y"+n, "ymm"+n)
		addName("Z"+n, "zmm"+n)
	}
	for i := 0; i <= 7; i++ {
		n := strconv.Itoa(i)
		add(x86asm.M0+x86asm.Reg(i), "M"+n, "", -1, "mm"+n)
		addName("K"+n, "k"+n)
	}
}

func plan9Register(name string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, "%")))
	if key == "" {
		return "", fmt.Errorf("empty register")
	}
	if reg, ok := amd64Registers[key]; ok {
		return reg.plan9, nil
	}
	return "", fmt.Errorf("unsupported register %q", name)
}

func intelRegWidth(arg string) (string, bool) {
	key := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(arg, "%")))
	if reg, ok := amd64Registers[key]; ok && reg.width != "" {
		return reg.width, true
	}
	return "", false
}

func reservedRegNumber(name string) (int, bool) {
	key := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, "%")))
	reg, ok := amd64Registers[key]
	return reg.number, ok && reg.number >= 12 && reg.number <= 15
}

func reservedRegMask(reg string) uint64 {
	if n, ok := reservedRegNumber(reg); ok {
		return 1 << n
	}
	return 0
}

func reservedMask(args []string) uint64 {
	var mask uint64
	for _, arg := range args {
		for _, token := range strings.FieldsFunc(arg, func(r rune) bool {
			return (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '%'
		}) {
			mask |= reservedRegMask(token)
		}
	}
	return mask
}

func fixedRegMask(regs []string) uint64 {
	var mask uint64
	for _, reg := range regs {
		mask |= reservedRegMask(reg)
	}
	return mask
}

func plan9Immediate(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "+")
	if v == "" {
		return "$0"
	}
	if asmutil.IsNumeric(v) {
		return "$" + v
	}
	return "$" + asmutil.AddSB(v)
}

func formatMemory(base, index, scale, disp string) (string, error) {
	baseReg := ""
	if base != "" {
		reg, err := x86MemoryRegister(base)
		if err != nil {
			return "", err
		}
		baseReg = reg
	}
	indexText := ""
	if index != "" {
		reg, err := x86MemoryRegister(index)
		if err != nil {
			return "", err
		}
		if scale == "" {
			scale = "1"
		}
		if scale != "" {
			switch scale {
			case "1", "2", "4", "8":
			default:
				return "", fmt.Errorf("unsupported scale %q", scale)
			}
		}
		indexText = "(" + reg + "*" + scale + ")"
	}
	if baseReg == "" {
		return disp + indexText, nil
	}
	return disp + "(" + baseReg + ")" + indexText, nil
}

func x86MemoryRegister(name string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, "%")))
	reg, ok := amd64Registers[key]
	if !ok || reg.number < 0 || reg.width == "B" {
		return "", fmt.Errorf("unsupported memory register %q", name)
	}
	return reg.plan9, nil
}

func containsELFReloc(s string) bool {
	return strings.Contains(s, "@")
}

func isIntelImmediate(v string) bool {
	return asmutil.IsNumeric(v)
}
