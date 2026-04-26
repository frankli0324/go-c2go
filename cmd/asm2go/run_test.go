package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	asmconv "github.com/frankli0324/go-c2go/internal/asm"
)

func TestRunHostVariant(t *testing.T) {
	tc := hostAsmCase(t, "sample")
	out := runCase(t, "sample.s", tc, "-syntax", "auto")
	mustContain(t, out, append([]string{"#include \"textflag.h\""}, tc.checks...)...)
	mustNotContain(t, out, tc.unwanted...)
}

func TestRunHostVariantWritesExplicitOutputFile(t *testing.T) {
	tc := hostAsmCase(t, "explicit")
	dir, src := writeAsm(t, "sample.s", tc.source)
	outPath := filepath.Join(dir, "translated.s")
	runOK(t, "-src", src, "-syntax", "auto", "-o", outPath)
	out := read(t, outPath)
	mustContain(t, out, append([]string{"#include \"textflag.h\""}, tc.checks...)...)
	mustNotContain(t, out, tc.unwanted...)
}

func TestRunPlan9VariantNotImplemented(t *testing.T) {
	_, src := writeAsm(t, "sample.s", "TEXT ·foo(SB), NOSPLIT, $0\n\tRET\n")
	if err := run([]string{"-src", src, "-syntax", "plan9"}); err == nil {
		t.Fatal("expected error for unimplemented plan9 variant")
	}
}

func TestRunKeepsLocalBranchTargets(t *testing.T) {
	tc := hostAsmCase(t, "branch")
	out := runCase(t, "branch.s", tc, "-syntax", "auto")
	mustContain(t, out, tc.checks...)
	mustNotContain(t, out, tc.unwanted...)
}

type asmCase struct {
	source   string
	checks   []string
	unwanted []string
}

func hostAsmCase(t *testing.T, name string) asmCase {
	t.Helper()
	cases := map[string]map[string]asmCase{
		asmconv.ArchARM64: {
			"sample":   {".globl\t_add\n_add:\n\tadd\tw0, w0, w1\n\tret\n", []string{"// .globl\t_add", "TEXT _add(SB), NOSPLIT|NOFRAME, $0", "ADDW R1, R0, R0", "RET"}, []string{"_add:\n"}},
			"explicit": {"main:\n\tmov\tx1, x0\n\tbl\tprintf\n\tret\n", []string{"TEXT main(SB), NOSPLIT|NOFRAME, $0", "MOVD R0, R1", "CALL printf(SB)", "RET"}, []string{"main:\n"}},
			"branch":   {".globl\t_branch\n_branch:\n\tcmp\tw0, #0\n\tb.eq\t.Lzero\n\tbl\tprintf\n.Lzero:\n\tret\n", []string{"TEXT _branch(SB), NOSPLIT|NOFRAME, $0", "BEQ Lzero", "CALL printf(SB)", "Lzero:"}, []string{".Lzero", "Lzero(SB)"}},
		},
		asmconv.ArchAMD64: {
			"sample":   {".globl\t_add\n_add:\n\tleal\t(%rdi,%rsi), %eax\n\tretq\n", []string{"// .globl\t_add", "TEXT _add(SB), NOSPLIT|NOFRAME, $0", "RET"}, []string{"_add:\n"}},
			"explicit": {"main:\n\tmovq\t%rsp, %rbp\n\tmovl\t$42, -4(%rbp)\n\tcallq\tprintf\n\tretq\n", []string{"TEXT main(SB), NOSPLIT|NOFRAME, $0", "MOVQ SP, BP", "MOVL $42, -4(BP)", "CALL printf(SB)", "RET"}, []string{"main:\n"}},
			"branch":   {".globl\t_branch\n_branch:\n\tcmpl\t$0, %edi\n\tje\t.Lzero\n\tcallq\tprintf\n.Lzero:\n\tretq\n", []string{"TEXT _branch(SB), NOSPLIT|NOFRAME, $0", "JE Lzero", "CALL printf(SB)", "Lzero:"}, []string{".Lzero", "Lzero(SB)"}},
		},
	}
	byArch, ok := cases[runtime.GOARCH]
	if !ok {
		t.Skipf("unsupported host architecture %s", runtime.GOARCH)
	}
	return byArch[name]
}

func runCase(t *testing.T, file string, tc asmCase, args ...string) string {
	t.Helper()
	dir, src := writeAsm(t, file, tc.source)
	runOK(t, append([]string{"-src", src}, args...)...)
	return read(t, filepath.Join(dir, strings.TrimSuffix(file, filepath.Ext(file))+"_"+runtime.GOARCH+".s"))
}

func writeAsm(t *testing.T, name, body string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)), 0o644); err != nil {
		t.Fatalf("write asm source: %v", err)
	}
	return dir, path
}

func read(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	return string(body)
}

func runOK(t *testing.T, args ...string) {
	t.Helper()
	if err := run(args); err != nil {
		t.Fatalf("run() failed: %v", err)
	}
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
