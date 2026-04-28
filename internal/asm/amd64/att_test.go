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
shrdq $13, %rbx, %rax
vpshufd $78, %xmm0, %xmm1
pxor %xmm1, %xmm0
`)
	mustContain(t, out,
		"LEAQ foo(SB), AX",
		"LEAQ foo(SB), AX",
		"MOVQ (AX), CX",
		"SHLQ $1, AX",
		"SHRQ $13, BX, AX",
		"VPSHUFD $78, X0, X1",
		"PXOR X1, X0",
	)
}

func TestPushPopFrameMarkers(t *testing.T) {
	att := translateATT(t, `
pushq %rbp
popq %rbp
`)
	mustContain(t, att, "// c2go: frame 8", "MOVQ BP, 0(SP)", "MOVQ 0(SP), BP")

	intel := translateIntel(t, `
push rbp
pop rbp
`)
	mustContain(t, intel, "// c2go: frame 8", "MOVQ BP, 0(SP)", "MOVQ 0(SP), BP")
}

func TestReservedRegistersNeedSave(t *testing.T) {
	if out, bad := new(ATT).TranslateInstruction("", "movq %r12, %rax"); !bad {
		t.Fatalf("ATT allowed unsaved reserved register: %s", out)
	}
	var att ATT
	for _, line := range []string{"pushq %r12", "movq %r12, %rax", "popq %r12"} {
		if out, bad := att.TranslateInstruction("", line); bad {
			t.Fatalf("ATT unsupported %q: %s", line, out)
		}
	}
	if out, bad := att.TranslateInstruction("", "movq %r12, %rax"); !bad {
		t.Fatalf("ATT allowed restored reserved register: %s", out)
	}
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
shld rax, rbx, 13
`)
	mustContain(t, intel, "SETGT AL", "SETLS 8(SP)", "IMUL3L $7, SI, AX", "SETLT R8B", "SHLQ $13, BX, AX")
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

func TestCompareOperandOrder(t *testing.T) {
	att := translateATT(t, `
cmpq %r10, %rax
ja Ldone
`)
	mustContain(t, att, "CMPQ AX, R10", "JA Ldone")
	mustNotContain(t, att, "CMPQ R10, AX")

	intel := translateIntel(t, `
cmp rax, r10
ja Ldone
`)
	mustContain(t, intel, "CMPQ AX, R10", "JA Ldone")
	mustNotContain(t, intel, "CMPQ R10, AX")
}

func TestATTCLTQ(t *testing.T) {
	out := translateATT(t, `cltq`)
	mustContain(t, out, "MOVLQSX AX, AX")
	mustNotContain(t, out, "CLTQ")
}

func TestRejectsELFRelocations(t *testing.T) {
	for _, line := range []string{
		"callq __builtin_rotateleft64@PLT",
		"movq foo@GOTPCREL(%rip), %rax",
		"leaq bar@GOTOFF+8(%rip), %rcx",
		"callq *foo@GOTPCREL(%rip)",
		"jmpq *bar@GOTPCREL(%rip)",
	} {
		out, bad := new(ATT).TranslateInstruction("", line)
		if !bad {
			t.Fatalf("ATT TranslateInstruction(%q) = %q, false, want unsupported", line, out)
		}
		mustContain(t, out, "// UNSUPPORTED: "+line)
	}
	for _, line := range []string{
		"call foo@PLT",
		"mov rax, [rip+foo@GOTPCREL]",
	} {
		out, bad := new(Intel).TranslateInstruction("", line)
		if !bad {
			t.Fatalf("Intel TranslateInstruction(%q) = %q, false, want unsupported", line, out)
		}
		mustContain(t, out, "// UNSUPPORTED: "+line)
	}
}

func TestIntelCDQE(t *testing.T) {
	out := translateIntel(t, `cdqe`)
	mustContain(t, out, "MOVLQSX AX, AX")

	var tr Intel
	unsupported, bad := tr.TranslateInstruction("", "cltq")
	if !bad {
		t.Fatalf("Intel cltq = %q, false, want unsupported", unsupported)
	}
}

func TestATTSuffixedConditionalBranches(t *testing.T) {
	out := translateATT(t, `
jeq .Ldone
jneq Lnext
`)
	mustContain(t, out, "JE .Ldone", "JNE Lnext")
	mustNotContain(t, out, "JEQ", "JNEQ", ".Ldone(SB)", "Lnext(SB)")
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
