package codegen

import "fmt"

type Output struct {
	Asm      string
	Go       string
	Fallback string
}

func GenerateBinding(src, asm, pkg, goos, arch string) (Output, error) {
	funcs, err := parseFunctions(src, goos, arch)
	if err != nil {
		return Output{}, err
	}
	if err := validateRegisterArgs(funcs, arch); err != nil {
		return Output{}, err
	}
	asm = wrapAssembly(asm, funcs, goos, arch)
	return Output{
		Asm:      asm,
		Go:       renderDecls(pkg, arch, funcs),
		Fallback: renderFallback(pkg, funcs),
	}, nil
}

func validateRegisterArgs(funcs []funcSpec, arch string) error {
	regs := argRegs(arch)
	if len(regs) == 0 {
		return fmt.Errorf("unsupported arch %q", arch)
	}
	for _, fn := range funcs {
		n := cArgCount(fn.Params)
		if n > len(regs) {
			return fmt.Errorf("%s needs %d integer registers; only %d are supported on %s", fn.CName, n, len(regs), arch)
		}
	}
	return nil
}
