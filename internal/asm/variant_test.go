package asm

import (
	"errors"
	"strings"
	"testing"
)

func TestResolveNormalizesSyntax(t *testing.T) {
	if got := Resolve(""); got != "" {
		t.Fatalf("Resolve(default) = %q, want empty", got)
	}
	if got := Resolve(" Intel "); got != Intel {
		t.Fatalf("Resolve(intel) = %q, want %q", got, Intel)
	}
}

func TestTranslateExamples(t *testing.T) {
	cases := []struct {
		name, syntax, arch, input string
		want, notWant             []string
	}{
		{"att", ATT, "amd64", `
# AT&T example
main:
    movq %rsp, %rbp
    movl $42, -4(%rbp)
    cmpl $64, %esi
    movss %xmm0, 8(%rsp)
    call printf
    addps %xmm1, %xmm2
    vaddps %ymm0, %ymm1, %ymm2
    ret
`, []string{"main:", "MOVQ SP, BP", "MOVL $42, -4(BP)", "CMPL SI, $64", "MOVSS X0, 8(SP)", "CALL printf(SB)", "ADDPS X1, X2", "VADDPS Y0, Y1, Y2", "RET"}, []string{"AT&T example"}},
		{"intel", Intel, "amd64", `
; Intel example
main:
    mov rbp, rsp
    mov dword ptr [rbp-4], 42
    movss dword ptr [rsp+8], xmm0
    call printf
    addps xmm2, xmm1
    vaddps ymm2, ymm0, ymm1
    ret
`, []string{"main:", "MOVQ SP, BP", "MOVL $42, -4(BP)", "MOVSS X0, 8(SP)", "CALL printf(SB)", "ADDPS X1, X2", "VADDPS Y0, Y1, Y2", "RET"}, []string{"Intel example"}},
		{"arm64", Auto, "arm64", `
_add:
    add w0, w1, w0
    ret
_mix:
    mov x8, #51847
    movk x8, #34283, lsl #16
    mul x0, x0, x8
    ret
_first:
    cmp w1, #1
    b.lt LBB2_2
    ldrsb w0, [x0]
    b LBB2_3
LBB2_2:
    mov w0, #0
LBB2_3:
    bl helper
    ret
`, []string{"_add:", "ADDW R0, R1, R0", "RET", "MOVD $51847, R8", "MOVK $(34283<<16), R8", "MUL R8, R0, R0", "CMPW $1, R1", "BLT LBB2_2", "MOVB (R0), R0", "JMP LBB2_3", "MOVW $0, R0", "CALL helper(SB)"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { checkTranslate(t, tc.syntax, tc.arch, tc.input, tc.want, tc.notWant) })
	}
}

func TestTranslateRejectsUnsupportedVariants(t *testing.T) {
	for _, syntax := range []string{ATT, Intel, Plan9} {
		_, err := Translate("ret\n", Context{Syntax: syntax, Arch: "arm64"})
		if err == nil || !strings.Contains(err.Error(), `is not supported for arm64`) {
			t.Fatalf("unexpected arm64 error for %s: %v", syntax, err)
		}
	}
	_, err := Translate("TEXT foo(SB),$0\n", Context{Syntax: Plan9, Arch: "amd64"})
	if err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("unexpected plan9 error: %v", err)
	}
}

func TestTranslateAddressingAndSIMD(t *testing.T) {
	checkTranslate(t, ATT, "amd64", `
movq foo(%rip), %rax
movq 8(%rip), %rax
vpaddq foo(%rip), %ymm1, %ymm0
`, []string{"MOVQ $foo(SB), AX", "MOVQ $8, AX", "VPADDQ foo(SB), Y1, Y0"}, nil)
	checkTranslate(t, ATT, "amd64", `
vpxor %ymm7, %ymm10, %ymm10
vpor %ymm1, %ymm10, %ymm1
movdqa (%rax), %xmm0
movdqa %xmm0, 16(%rax)
movdqu (%rax), %xmm0
movdqu %xmm0, 16(%rax)
maskmovdqu %xmm1, %xmm2
movntdq %xmm0, (%rax)
vmovdqa (%rax), %ymm1
vmovdqu (%rax), %ymm1
vinserti128 $1, %xmm2, %ymm1, %ymm1
vextracti128 $1, %ymm1, %xmm1
vpbroadcastq foo(%rip), %ymm2
vpermq $54, (%rdi), %ymm10
vzeroupper
`, []string{"VPXOR Y7, Y10, Y10", "VPOR Y1, Y10, Y1", "MOVO (AX), X0", "MOVO X0, 16(AX)", "MOVOU (AX), X0", "MOVOU X0, 16(AX)", "MASKMOVOU X1, X2", "MOVNTO X0, (AX)", "VMOVDQA (AX), Y1", "VMOVDQU (AX), Y1", "VINSERTI128 $1, X2, Y1, Y1", "VEXTRACTI128 $1, Y1, X1", "VPBROADCASTQ foo(SB), Y2", "VPERMQ $54, (DI), Y10", "VZEROUPPER"}, nil)
	checkTranslate(t, Intel, "amd64", `
movdqa xmm0, [rax]
movdqa [rax+16], xmm0
movdqu xmm1, [rax]
movdqu [rax+16], xmm1
movntdq [rax], xmm0
vmovdqa ymm2, [rax]
vmovdqu ymm3, [rax]
`, []string{"MOVO (AX), X0", "MOVO X0, 16(AX)", "MOVOU (AX), X1", "MOVOU X1, 16(AX)", "MOVNTO X0, (AX)", "VMOVDQA (AX), Y2", "VMOVDQU (AX), Y3"}, nil)
	checkTranslate(t, ATT, "amd64", `
movsbl (%rdi), %eax
movzbl (%rsi), %eax
movslq %eax, %rax
movabsq $-4417276706812531889, %rax
cmovgl %esi, %ecx
cmovbq %r8, %rbx
`, []string{"MOVBLSX (DI), AX", "MOVBLZX (SI), AX", "MOVLQSX AX, AX", "MOVQ $-4417276706812531889, AX", "CMOVLGT SI, CX", "CMOVQCS R8, BX"}, nil)
	checkTranslate(t, Intel, "amd64", "mov rax, [rip+foo]\nmov rax, [rip+8]\n", []string{"MOVQ $foo(SB), AX", "MOVQ $8, AX"}, nil)
}

func TestTranslateMetadataAndComments(t *testing.T) {
	out, err := Translate(strings.TrimSpace(`
.file 1 "x.c"
.loc 1 1 0
.cfi_startproc
.ident "clang"
.addrsig
.build_version macos, 15, 0 sdk_version 26, 2
.subsections_via_symbols
// %bb.0:
.globl _foo
.section __TEXT,__text
.p2align 4, 0x90
.byte 1
.short 2
.word 3
.long 4
.xword 5
.unknown meta
ret
`)+"\n", Context{Syntax: ATT, Arch: "amd64"})
	if count := unsupportedCount(t, err); count != 1 {
		t.Fatalf("UnsupportedError.Count = %d, want 1\n%s", count, out)
	}
	mustContain(t, out, "// .globl _foo", "// .section __TEXT,__text", "PCALIGN $16", ".byte 1", ".short 2", ".word 3", ".long 4", ".xword 5", "// UNSUPPORTED: .unknown meta", "RET")
	mustNotContain(t, out, ".cfi_startproc", ".file 1 \"x.c\"", ".loc 1 1 0", ".ident", ".addrsig", ".build_version", ".subsections_via_symbols", "%bb.0")
	checkTranslate(t, ATT, "amd64", "movq %rsp, %rbp # save frame\n", []string{"MOVQ SP, BP"}, []string{"save frame"})
	checkTranslate(t, Intel, "amd64", "mov rbp, rsp ; intel form\n", []string{"MOVQ SP, BP"}, []string{"intel form"})
}

func TestTranslateContextControlsPCALIGN(t *testing.T) {
	src := ".p2align 4, 0x90\nret\n"
	out, err := Translate(src, Context{Syntax: ATT, Arch: "amd64", GoVersion: "go1.21"})
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, out, "// .p2align 4, 0x90")
	mustNotContain(t, out, "PCALIGN")

	out, err = Translate(src, Context{Syntax: ATT, Arch: "amd64", GoVersion: "go1.21.13"})
	if err != nil {
		t.Fatal(err)
	}
	mustNotContain(t, out, "PCALIGN")

	out, err = Translate(src, Context{Syntax: ATT, Arch: "amd64", GoVersion: "go1.26"})
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, out, "PCALIGN $16")
}

func TestTranslateMemoryAddressingForms(t *testing.T) {
	checkTranslate(t, ATT, "amd64", `
movq (%rax,%rcx,4), %rdx
movq foo+8(%rbp), %rax
`, []string{"MOVQ (AX)(CX*4), DX", "MOVQ foo+8(BP), AX"}, nil)
	checkTranslate(t, Intel, "amd64", `
mov rax, [rbp-4]
mov rax, [rax+rcx*8+16]
`, []string{"MOVQ -4(BP), AX", "MOVQ 16(AX)(CX*8), AX"}, nil)
}

func TestTranslateSIMDMnemonics(t *testing.T) {
	checkTranslate(t, ATT, "amd64", `
pshufb %xmm1, %xmm2
cvttss2si %xmm0, %eax
`, []string{"PSHUFB X1, X2", "CVTTSS2SI X0, AX"}, nil)
	checkTranslate(t, Intel, "amd64", `
cvttss2si eax, xmm0
vcvtss2sd xmm2, xmm0, xmm1
`, []string{"CVTTSS2SI X0, AX", "VCVTSS2SD X0, X1, X2"}, nil)
}

func TestTranslateBranchTargets(t *testing.T) {
	checkTranslate(t, ATT, "amd64", `
main:
    je .LBB0_1
    jmp Ltmp0
    callq printf
.LBB0_1:
    retq
`, []string{"JE .LBB0_1", "JMP Ltmp0", "CALL printf(SB)"}, []string{".LBB0_1(SB)", "Ltmp0(SB)"})
	checkTranslate(t, Intel, "amd64", `
main:
    jne .LBB1_2
    jmp Ltmp1
    call printf
.LBB1_2:
    ret
`, []string{"JNE .LBB1_2", "JMP Ltmp1", "CALL printf(SB)"}, []string{".LBB1_2(SB)", "Ltmp1(SB)"})
}

func TestTranslateIndirectBranchTargets(t *testing.T) {
	checkTranslate(t, ATT, "amd64", `
main:
    callq *%rax
    jmp *8(%rbp)
`, []string{"CALL AX", "JMP 8(BP)"}, []string{"*%rax(SB)", "*8(%rbp)(SB)"})
	checkTranslate(t, Intel, "amd64", `
main:
    call rax
    jmp qword ptr [rbp+8]
`, []string{"CALL AX", "JMP 8(BP)"}, []string{"rax(SB)", "[rbp+8](SB)"})
}

func checkTranslate(t *testing.T, syntax, arch, input string, want, notWant []string, ctx ...Context) string {
	t.Helper()
	asmCtx := Context{Syntax: syntax, Arch: arch}
	if len(ctx) > 0 {
		asmCtx = ctx[0]
	}
	out, err := Translate(strings.TrimSpace(input)+"\n", asmCtx)
	if err != nil {
		t.Fatalf("Translate(%s/%s) error = %v", syntax, arch, err)
	}
	mustContain(t, out, want...)
	mustNotContain(t, out, notWant...)
	return out
}

func unsupportedCount(t *testing.T, err error) int {
	t.Helper()
	var unsupported UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("Translate() error = %v, want UnsupportedError", err)
	}
	return unsupported.Count
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
	for _, unwanted := range checks {
		if strings.Contains(text, unwanted) {
			t.Fatalf("output contains unwanted %q\n%s", unwanted, text)
		}
	}
}
