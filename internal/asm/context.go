package asm

import (
	"go/version"
)

func (ctx Context) supportsPCALIGN() bool {
	if ctx.Arch != ArchAMD64 {
		return true
	}
	if !version.IsValid(ctx.GoVersion) {
		return true
	}
	return version.Compare(version.Lang(ctx.GoVersion), "go1.22") >= 0
}
