package arm64

import (
	"strings"

	"github.com/frankli0324/go-c2go/internal/asm/asmutil"
)

type Translator struct {
	fullAddr map[string]string
}

func (*Translator) CommentPrefix() string {
	return ";"
}

func (t *Translator) TranslateInstruction(indent, line string) (string, bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return indent, false
	}
	op := strings.ToLower(fields[0])
	args := asmutil.SplitOperands(strings.TrimSpace(strings.TrimPrefix(line, fields[0])))
	out, ok, err := translateOp(t, op, args)
	if err != nil || !ok {
		return indent + "// UNSUPPORTED: " + line, true
	}
	return indent + out, false
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
