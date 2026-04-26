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
mov.16b v0, v1
add.2d v0, v0, v1
`)
	mustContain(t, out,
		"MOVD.W -16(RSP), R0",
		"FMOVQ F0, (R1)",
		"FMOVQ (R2), F1",
		"MOVD (R4)(R5), R3",
		"VMOV V1.B16, V0.B16",
		"VADD V1.D2, V0.D2, V0.D2",
	)
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
