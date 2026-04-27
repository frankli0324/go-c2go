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

func TestPlan9RegisterNames(t *testing.T) {
	cases := map[string]string{
		"al":    "AL",
		"ah":    "AH",
		"sil":   "SIB",
		"spl":   "SPB",
		"r8b":   "R8B",
		"r15b":  "R15B",
		"xmm31": "X31",
		"ymm31": "Y31",
		"zmm31": "Z31",
		"k7":    "K7",
		"mm7":   "M7",
	}
	for input, want := range cases {
		got, err := plan9Register(input)
		if err != nil || got != want {
			t.Fatalf("plan9Register(%q) = %q, %v, want %q", input, got, err, want)
		}
	}
}

func TestIntelByteRegisterWidth(t *testing.T) {
	for _, reg := range []string{"al", "ah", "sil", "spl", "r8b", "r15b"} {
		got, ok := intelRegWidth(reg)
		if !ok || got != "B" {
			t.Fatalf("intelRegWidth(%q) = %q, %v, want B, true", reg, got, ok)
		}
	}
}

func TestConditionSetAndIMUL3(t *testing.T) {
	att := translateATT(t, `
sete %al
setne 8(%rsp)
imull $7, %esi, %eax
setb %r9b
`)
	mustContain(t, att, "SETEQ AL", "SETNE 8(SP)", "IMUL3L $7, SI, AX", "SETCS R9B")

	intel := translateIntel(t, `
setg al
setbe byte ptr [rsp+8]
imul eax, esi, 7
setl r8b
`)
	mustContain(t, intel, "SETGT AL", "SETLS 8(SP)", "IMUL3L $7, SI, AX", "SETLT R8B")
}

func TestUnsignedWordLoadsStayZeroExtended(t *testing.T) {
	att := translateATT(t, `
movzbl (%rdi), %eax
movl (%rsi), %eax
`)
	mustContain(t, att, "MOVBLZX (DI), AX", "MOVL (SI), AX")
	mustNotContain(t, att, "MOVBLSX", "MOVLQSX")

	intel := translateIntel(t, `
movzx eax, byte ptr [rdi]
mov eax, dword ptr [rsi]
`)
	mustContain(t, intel, "MOVZXB (DI), AX", "MOVL (SI), AX")
	mustNotContain(t, intel, "MOVSXL", "MOVLQSX")
}

func TestIntelAVXWhitelistReorder(t *testing.T) {
	out, unsupported := translateIntelAllowUnsupported(t, `
vaddps ymm2, ymm0, ymm1
vpaddq ymm2, ymm0, ymm1
vpsubd xmm3, xmm4, xmm5
vpcmpeqd xmm6, xmm7, xmm8
vpsllq xmm9, xmm10, xmm11
vfoobar xmm2, xmm0, xmm1
`)
	if unsupported != 1 {
		t.Fatalf("unsupported = %d, want 1\n%s", unsupported, out)
	}
	mustContain(t, out,
		"VADDPS Y0, Y1, Y2",
		"VPADDQ Y0, Y1, Y2",
		"VPSUBD X4, X5, X3",
		"VPCMPEQD X7, X8, X6",
		"VPSLLQ X10, X11, X9",
		"// UNSUPPORTED: vfoobar xmm2, xmm0, xmm1",
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

func translateIntelAllowUnsupported(t *testing.T, src string) (string, int) {
	t.Helper()
	var out []string
	var unsupported int
	var tr Intel
	for _, line := range strings.Split(strings.TrimSpace(src), "\n") {
		converted, bad := tr.TranslateInstruction("", strings.TrimSpace(line))
		if bad {
			unsupported++
		}
		out = append(out, converted)
	}
	return strings.Join(out, "\n"), unsupported
}

func translateIntel(t *testing.T, src string) string {
	t.Helper()
	var out []string
	var tr Intel
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

func mustNotContain(t *testing.T, text string, checks ...string) {
	t.Helper()
	for _, bad := range checks {
		if strings.Contains(text, bad) {
			t.Fatalf("output contains %q\n%s", bad, text)
		}
	}
}
