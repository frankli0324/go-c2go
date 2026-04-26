package amd64

import (
	"fmt"

	"github.com/frankli0324/go-c2go/internal/asm/asmutil"
)

func Resolve(variant string) (asmutil.Translator, error) {
	switch variant {
	case "", "auto":
		return ATT{}, nil
	case "att":
		return ATT{}, nil
	case "intel":
		return Intel{}, nil
	case "plan9":
		return nil, fmt.Errorf("asm syntax %q is not implemented yet", variant)
	default:
		return nil, fmt.Errorf("unsupported asm syntax %q for amd64", variant)
	}
}
