package arm64

import (
	"fmt"

	"github.com/frankli0324/go-c2go/internal/asm/asmutil"
)

func Resolve(variant string, trustFixedRegs []string) (asmutil.Translator, error) {
	if variant == "" || variant == "auto" {
		return &Translator{fullAddr: make(map[string]string), trustFixedRegs: trustFixedRegs}, nil
	}
	return nil, fmt.Errorf("asm syntax %q is not supported for arm64", variant)
}
