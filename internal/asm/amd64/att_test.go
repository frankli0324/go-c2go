package amd64

import (
	"strings"
	"testing"
)

func TestATTRecentForms(t *testing.T) {
	out := translateATT(t, `
leaq foo(%rip), %rax
leaq $foo, %rax
movq (rax), %rcx
shlq %rax
vpshufd $78, %xmm0, %xmm1
pxor %xmm1, %xmm0
`)
	mustContain(t, out,
		"LEAQ foo(SB), AX",
		"LEAQ foo(SB), AX",
		"MOVQ (AX), CX",
		"SHLQ $1, AX",
		"VPSHUFD $78, X0, X1",
		"PXOR X1, X0",
	)
}

func translateATT(t *testing.T, src string) string {
	t.Helper()
	var out []string
	var tr ATT
	for _, line := range strings.Split(strings.TrimSpace(src), "\n") {
		converted, bad := tr.TranslateInstruction("", strings.TrimSpace(line))
		if bad {
			t.Fatalf("unsupported line %q -> %s", line, converted)
		}
		out = append(out, converted)
	}
	return strings.Join(out, "\n")
}

func mustContain(t *testing.T, text string, checks ...string) {
	t.Helper()
	for _, want := range checks {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q\n%s", want, text)
		}
	}
}
