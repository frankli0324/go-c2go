package arm64

import (
	"strings"
	"testing"
)

func TestPageOffsetsRequireResolvedBase(t *testing.T) {
	out := translateARM64(t, `
adrp x8, .LCPI0_0
ldr q1, [x8, :lo12:.LCPI0_0]
adrp x9, _foo@PAGE
ldr x0, [x9, _foo@PAGEOFF]
`)
	mustContain(t, out,
		"MOVD $.LCPI0_0(SB), R8",
		"FMOVQ (R8), F1",
		"MOVD $_foo(SB), R9",
		"MOVD (R9), R0",
	)

	var tr Translator
	line, bad := tr.TranslateInstruction("", "ldr x0, [x8, :lo12:.LCPI0_0]")
	if !bad {
		t.Fatalf("TranslateInstruction succeeded, want unsupported: %s", line)
	}
	mustContain(t, line, "// UNSUPPORTED: ldr x0, [x8, :lo12:.LCPI0_0]")
}

func TestPageAddState(t *testing.T) {
	out := translateARM64(t, `
adrp x8, foo
add x8, x8, :lo12:foo
ldr x0, [x8]
adrp x10, bar@PAGE+8
add x11, x10, bar@PAGEOFF+8
ldr x1, [x11, bar@PAGEOFF+8]
add x12, x13, :lo12:baz
`)
	mustContain(t, out,
		"MOVD $foo(SB), R8",
		"// add x8, x8, :lo12:foo",
		"MOVD (R8), R0",
		"MOVD $bar+8(SB), R10",
		"MOVD R10, R11",
		"MOVD (R11), R1",
		"MOVD $baz(SB), R12",
	)
}

func TestMemoryAndVectorForms(t *testing.T) {
	out := translateARM64(t, `
ldr x0, [sp, #-16]!
str q0, [x1]
ldr q1, [x2]
ldr x3, [x4, x5]
stp x1, x2, [sp]
ldp x3, x4, [sp]
mov.16b v0, v1
add.2d v0, v0, v1
`)
	mustContain(t, out,
		"MOVD.W -16(RSP), R0",
		"FMOVQ F0, (R1)",
		"FMOVQ (R2), F1",
		"MOVD (R4)(R5), R3",
		"STP (R1, R2), (RSP)",
		"LDP (RSP), (R3, R4)",
		"VMOV V1.B16, V0.B16",
		"VADD V1.D2, V0.D2, V0.D2",
	)
}

func TestUnsignedLoads(t *testing.T) {
	out := translateARM64(t, `
ldrb w0, [x1]
ldrh w2, [x3]
ldr w4, [x5]
ldur w6, [x7, #4]
ldr w8, [x9], #4
ldrsw x10, [x11]
`)
	mustContain(t, out,
		"MOVBU (R1), R0",
		"MOVHU (R3), R2",
		"MOVWU (R5), R4",
		"MOVWU 4(R7), R6",
		"MOVWU.P 4(R9), R8",
		"MOVW (R11), R10",
	)
	mustNotContain(t, out, "MOVW (R5), R4", "MOVW 4(R7), R6", "MOVW.P (R9), R8")
}

func TestCBZBranches(t *testing.T) {
	out := translateARM64(t, `
cbz w0, LBB0_2
cbnz x1, .LBB0_3
tbz w2, #3, LBB0_4
tbnz x3, #4, .LBB0_5
`)
	mustContain(t, out,
		"CBZ R0, LBB0_2",
		"CBNZ R1, .LBB0_3",
		"TBZ $3, R2, LBB0_4",
		"TBNZ $4, R3, .LBB0_5",
	)
	mustNotContain(t, out, "LBB0_2(SB)", ".LBB0_3(SB)", "LBB0_4(SB)", ".LBB0_5(SB)")
}

func TestScalarOpCoverage(t *testing.T) {
	out := translateARM64(t, `
tst x0, x1
tst x0, x1, lsl #3
cmn x2, #7
cset w3, lt
sdiv x0, x1, x2
udiv w3, w4, w5
extr x22, x5, x16, #33
extr w1, w2, w3, #7
msub x6, x7, x8, x9
adds w10, w11, #1
adcs x12, x13, x14
sbc x15, x16, x17
sbcs w4, w19, w20
neg w21, w22
negs x23, x24
ubfx w25, w24, #2, #6
sbfx x6, x25, #3, #9
adr x24, Ltmp0
ldrsw x9, [x10]
mov x16, #2147483648
`)
	mustContain(t, out,
		"TST R1, R0",
		"TST R1<<3, R0",
		"CMN $7, R2",
		"CSETW LT, R3",
		"SDIV R2, R1, R0",
		"UDIVW R5, R4, R3",
		"EXTR $33, R16, R5, R22",
		"EXTRW $7, R3, R2, R1",
		"MSUB R8, R9, R7, R6",
		"ADDSW $1, R11, R10",
		"ADCS R14, R13, R12",
		"SBC R17, R16, R15",
		"SBCSW R20, R19, R4",
		"NEGW R22, R21",
		"NEGS R24, R23",
		"UBFXW $2, R24, $6, R25",
		"SBFX $3, R25, $9, R6",
		"ADR Ltmp0, R24",
		"MOVW (R10), R9",
		"MOVD $2147483648, R16",
	)
}

func TestReservedRegistersRejected(t *testing.T) {
	for _, line := range []string{
		"add x0, x18, x1",
		"add x26, x0, x1",
		"ldr x0, [x27]",
		"str x28, [x0]",
		"adr x29, Ltmp0",
	} {
		var tr Translator
		out, bad := tr.TranslateInstruction("", line)
		if !bad {
			t.Fatalf("TranslateInstruction(%q) = %q, want unsupported", line, out)
		}
		mustContain(t, out, "// UNSUPPORTED: "+line)
	}
}

func TestPairSaveRestore(t *testing.T) {
	out := translateARM64(t, `
stp x26, x25, [sp, #16]
ldp x29, x30, [sp, #80]
add x30, x4, #1
cmp x30, x1
csel x30, x24, x10, lo
ldrb w30, [x30, #1]
orr x25, x25, x30, lsl #8
`)
	mustContain(t, out,
		"STP (R26, R25), 16(RSP)",
		"LDP 80(RSP), (R29, R30)",
		"ADD $1, R4, R30",
		"CMP R1, R30",
		"CSEL LO, R24, R10, R30",
		"MOVBU 1(R30), R30",
		"ORR R30<<8, R25, R25",
	)
}

func TestSavedReservedRegisterAllowedUntilRestore(t *testing.T) {
	var tr Translator
	for _, line := range []string{
		"stp x26, x25, [sp, #16]",
		"add x0, x26, x1",
		"ldp x26, x25, [sp, #24]",
		"add x0, x26, x1",
		"ldp x26, x25, [sp, #16]",
	} {
		if out, bad := tr.TranslateInstruction("", line); bad {
			t.Fatalf("TranslateInstruction(%q) unsupported: %s", line, out)
		}
	}
	if out, bad := tr.TranslateInstruction("", "add x0, x26, x1"); !bad {
		t.Fatalf("TranslateInstruction allowed restored reserved register: %s", out)
	}
}

func TestDropFixedReservedSaveRestore(t *testing.T) {
	tr := Translator{trustFixedRegs: []string{"x26", "x27", "x28"}}
	for _, line := range []string{
		"stp x26, x25, [sp, #16]",
		"add x0, x26, x1",
		"ldp x26, x25, [sp, #16]",
	} {
		out, bad := tr.TranslateInstruction("", line)
		if bad {
			t.Fatalf("TranslateInstruction(%q) unsupported: %s", line, out)
		}
		if strings.HasPrefix(line, "stp ") || strings.HasPrefix(line, "ldp ") {
			mustContain(t, out, "// c2go: dropped arm64 Go ABI reserved register save/restore: "+line)
		}
	}
}

func TestVectorOpLaneCoverage(t *testing.T) {
	out := translateARM64(t, `
add.8b v0, v1, v2
add.16b v0, v1, v2
add.4h v0, v1, v2
add.8h v0, v1, v2
add.2s v0, v1, v2
add.4s v0, v1, v2
add.2d v0, v1, v2
eor.8b v3, v4, v5
orr.8b v6, v7, v8
ext.8b v9, v10, v11, #1
ushll.8h v12, v13, #0
ushll2.8h v14, v15, #0
`)
	mustContain(t, out,
		"VADD V2.B8, V1.B8, V0.B8",
		"VADD V2.B16, V1.B16, V0.B16",
		"VADD V2.H4, V1.H4, V0.H4",
		"VADD V2.H8, V1.H8, V0.H8",
		"VADD V2.S2, V1.S2, V0.S2",
		"VADD V2.S4, V1.S4, V0.S4",
		"VADD V2.D2, V1.D2, V0.D2",
		"VEOR V5.B8, V4.B8, V3.B8",
		"VORR V8.B8, V7.B8, V6.B8",
		"VEXT $1, V11.B8, V10.B8, V9.B8",
		"VUSHLL $0, V13.B8, V12.H8",
		"VUSHLL2 $0, V15.B16, V14.H8",
	)
}

func translateARM64(t *testing.T, src string) string {
	t.Helper()
	var out []string
	tr := Translator{fullAddr: make(map[string]string)}
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
