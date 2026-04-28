package amd64

import (
	"fmt"
	"strings"
)

type frame struct {
	slots map[string]int
	size  int
}

type frameTranslator struct {
	frame          frame
	savedRegs      uint64
	trustFixedRegs []string
}

func (t *ATT) pushPop(op, reg, line string) (string, bool) {
	state := frameTranslator{frame: t.frame, savedRegs: t.savedRegs, trustFixedRegs: t.trustFixedRegs}
	out, ok := state.pushPop(op, reg, line)
	t.frame, t.savedRegs = state.frame, state.savedRegs
	return out, ok
}

func (t *Intel) pushPop(op, reg, line string) (string, bool) {
	state := frameTranslator{frame: t.frame, savedRegs: t.savedRegs, trustFixedRegs: t.trustFixedRegs}
	out, ok := state.pushPop(op, reg, line)
	t.frame, t.savedRegs = state.frame, state.savedRegs
	return out, ok
}

func (t *frameTranslator) pushPop(op, reg, line string) (string, bool) {
	mask := reservedRegMask(reg)
	trusted := mask != 0 && mask&^fixedRegMask(t.trustFixedRegs) == 0
	if strings.HasPrefix(op, "push") {
		t.savedRegs |= mask
		if trusted {
			return "// c2go: dropped amd64 Go ABI reserved register save/restore: " + line, true
		}
		return t.frame.push(reg), true
	}
	if trusted {
		t.savedRegs &^= mask
		return "// c2go: dropped amd64 Go ABI reserved register save/restore: " + line, true
	}
	out, ok := t.frame.pop(reg)
	if ok {
		t.savedRegs &^= mask
	}
	return out, ok
}

func (f *frame) push(reg string) string {
	if f.slots == nil {
		f.slots = make(map[string]int)
	}
	if _, ok := f.slots[reg]; !ok {
		f.slots[reg] = f.size
		f.size += 8
	}
	return fmt.Sprintf("// c2go: frame %d\nMOVQ %s, %d(SP)", f.size, reg, f.slots[reg])
}

func (f *frame) pop(reg string) (string, bool) {
	offset, ok := f.slots[reg]
	if !ok {
		return "", false
	}
	return fmt.Sprintf("MOVQ %d(SP), %s", offset, reg), true
}

func pushPopReg(op string, args []string) (string, bool) {
	if len(args) != 1 {
		return "", false
	}
	switch op {
	case "pushq", "push":
	case "popq", "pop":
	default:
		return "", false
	}
	reg := strings.TrimSpace(args[0])
	reg = strings.TrimPrefix(reg, "%")
	out, err := plan9Register(reg)
	return out, err == nil
}
