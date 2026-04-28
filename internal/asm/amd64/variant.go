package amd64

import (
	"fmt"

	"github.com/frankli0324/go-c2go/internal/asm/asmutil"
)

func Resolve(variant string, trustFixedRegs []string) (asmutil.Translator, error) {
	switch variant {
	case "", "auto":
		return &ATT{trustFixedRegs: trustFixedRegs}, nil
	case "att":
		return &ATT{trustFixedRegs: trustFixedRegs}, nil
	case "intel":
		return &Intel{trustFixedRegs: trustFixedRegs}, nil
	case "plan9":
		return nil, fmt.Errorf("asm syntax %q is not implemented yet", variant)
	default:
		return nil, fmt.Errorf("unsupported asm syntax %q for amd64", variant)
	}
}
