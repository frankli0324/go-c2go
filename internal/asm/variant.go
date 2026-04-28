package asm

import (
	"fmt"
	"strings"

	"github.com/frankli0324/go-c2go/internal/asm/amd64"
	"github.com/frankli0324/go-c2go/internal/asm/arm64"
	"github.com/frankli0324/go-c2go/internal/asm/asmutil"
)

const (
	Auto      = "auto"
	ATT       = "att"
	Intel     = "intel"
	Plan9     = "plan9"
	ArchAMD64 = "amd64"
	ArchARM64 = "arm64"
)

type translator = asmutil.Translator

type UnsupportedError struct {
	Count int
}

type Context struct {
	Syntax    string
	Arch      string
	GoVersion string
}

func (e UnsupportedError) Error() string {
	return fmt.Sprintf("%d unsupported asm line(s)", e.Count)
}

func Resolve(name string) string {
	return strings.TrimSpace(strings.ToLower(name))
}

func Translate(src string, ctx Context) (string, error) {
	src = normalizeCommon(src)
	v, err := ctx.translator()
	if err != nil {
		return "", err
	}
	out, unsupported := ctx.translateLines(src, v)
	if unsupported > 0 {
		return out, UnsupportedError{Count: unsupported}
	}
	return out, nil
}

func (ctx Context) translator() (translator, error) {
	variant := Resolve(ctx.Syntax)
	switch ctx.Arch {
	case ArchAMD64:
		return amd64.Resolve(variant)
	case ArchARM64:
		return arm64.Resolve(variant)
	default:
		return nil, fmt.Errorf("asm translation spec currently only supports amd64 and arm64, got %q", ctx.Arch)
	}
}

func normalizeCommon(src string) string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")
	return src
}
