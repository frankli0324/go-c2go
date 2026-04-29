package arm64

import (
	"strings"

	"github.com/frankli0324/go-c2go/internal/asm/asmutil"
)

type Translator struct {
	fullAddr       map[string]string
	stack          map[string]uint64
	trustFixedRegs []string
}

func (*Translator) CommentPrefix() string {
	return ";"
}

func (t *Translator) ResetState() {
	t.fullAddr = make(map[string]string)
	t.stack = nil
}

func (t *Translator) TranslateInstruction(line string) (string, bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", false
	}
	op := strings.ToLower(fields[0])
	args := asmutil.SplitOperands(strings.TrimSpace(strings.TrimPrefix(line, fields[0])))
	stackKey, stackMask, stackPair := stackPair(op, args)
	if !stackPair && reservedMask(args)&^t.savedMask() != 0 {
		return "// UNSUPPORTED: " + line, true
	}
	out, ok, err := translateOp(t, op, args)
	if err != nil || !ok {
		return "// UNSUPPORTED: " + line, true
	}
	if stackPair {
		if op == "stp" {
			if t.stack == nil {
				t.stack = make(map[string]uint64)
			}
			t.stack[stackKey] |= stackMask
		} else if mask := t.stack[stackKey] &^ stackMask; mask == 0 {
			delete(t.stack, stackKey)
		} else {
			t.stack[stackKey] = mask
		}
		if stackMask != 0 && stackMask&^fixedRegMask(t.trustFixedRegs) == 0 {
			return "// c2go: dropped arm64 Go ABI reserved register save/restore: " + line, false
		}
	}
	return out, false
}

func (t *Translator) savedMask() uint64 {
	var mask uint64
	for _, slot := range t.stack {
		mask |= slot
	}
	return mask
}

func stackPair(op string, args []string) (string, uint64, bool) {
	if op != "stp" && op != "ldp" || len(args) < 3 {
		return "", 0, false
	}
	mem := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(args[2]), " ", ""))
	if len(args) == 4 {
		mem = strings.TrimSuffix(mem, "]") + "," + strings.ToLower(strings.ReplaceAll(strings.TrimSpace(args[3]), " ", "")) + "]"
	}
	if !strings.HasPrefix(mem, "[sp") && !strings.Contains(mem, "(rsp)") {
		return "", 0, false
	}
	return mem, reservedMask(args[:2]), true
}

func reservedMask(args []string) uint64 {
	var mask uint64
	for _, arg := range args {
		for _, token := range strings.FieldsFunc(strings.ToLower(arg), func(r rune) bool {
			return (r < 'a' || r > 'z') && (r < '0' || r > '9')
		}) {
			if n, ok := reservedRegNumber(token); ok {
				mask |= 1 << n
			}
		}
	}
	return mask
}

func fixedRegMask(regs []string) uint64 {
	var mask uint64
	for _, reg := range regs {
		if n, ok := reservedRegNumber(reg); ok {
			mask |= 1 << n
		}
	}
	return mask
}

func (t *Translator) remember(reg, sym string) {
	if t.fullAddr == nil {
		t.fullAddr = make(map[string]string)
	}
	t.fullAddr[reg] = sym
}

func (t *Translator) clear(arg string) {
	reg, err := register(arg)
	if err == nil {
		delete(t.fullAddr, reg)
	}
}
