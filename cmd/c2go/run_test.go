package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	asmconv "github.com/frankli0324/go-c2go/internal/asm"
	"github.com/frankli0324/go-c2go/internal/codegen"
)

const sampleC = `
int add(int a, int b) { return a + b; }
int max2(int a, int b) { return a > b ? a : b; }
int abs1(int x) { return x < 0 ? -x : x; }
int sum3(int a, int b, int c) { return a + b + c; }
long add64(long a, long b) { return a + b; }
`

func TestRunHostArchToPlan9File(t *testing.T) {
	goos, arch := requireHostCompilerTarget(t)
	dir := t.TempDir()
	src := write(t, dir, "sample.c", sampleC)
	runOK(t, "-src", src, "-cc", "clang", "-arch", arch, "-syntax", "auto")
	text := read(t, filepath.Join(dir, "sample_"+arch+".s"))
	for _, name := range []string{"add", "max2", "abs1", "sum3", "add64"} {
		sym := codegen.CompilerSymbol(goos, name)
		mustContain(t, text, "TEXT "+sym+"(SB), NOSPLIT|NOFRAME, $0")
		mustNotContain(t, text, sym+":\n")
	}
	mustContain(t, text, "#include \"textflag.h\"", "// .globl", "RET")
	assertTranslatedArithmetic(t, arch, text)
	if !strings.Contains(text, "CALL") && strings.Contains(text, "printf") {
		t.Fatalf("unexpected call formatting\n%s", text)
	}
}

func TestRunHostArchWritesExplicitFile(t *testing.T) {
	goos, arch := requireHostCompilerTarget(t)
	dir := t.TempDir()
	src := write(t, dir, "sample.c", "int add(int a, int b) { return a + b; }\n")
	out := filepath.Join(dir, "translated.s")
	runOK(t, "-src", src, "-cc", "clang", "-arch", arch, "-syntax", "auto", "-o", out)
	text := read(t, out)
	mustContain(t, text, "#include \"textflag.h\"", "RET", "TEXT "+codegen.CompilerSymbol(goos, "add")+"(SB), NOSPLIT|NOFRAME, $0")
	assertTranslatedArithmetic(t, arch, text)
}

func TestRunAMD64ImmintrinFromClang(t *testing.T) {
	_, arch := requireHostCompilerTarget(t)
	if arch != asmconv.ArchAMD64 {
		t.Skipf("immintrin smoke uses amd64 SSE, got %s", arch)
	}
	dir := t.TempDir()
	src := write(t, dir, "immintrin.c", `
#include <immintrin.h>
void add4_intrin(float *dst, const float *a, const float *b) {
	__m128 av = _mm_loadu_ps(a), bv = _mm_loadu_ps(b);
	_mm_storeu_ps(dst, _mm_add_ps(av, bv));
}
`)
	out := filepath.Join(dir, "immintrin.s")
	runOK(t, "-src", src, "-cc", "clang", "-arch", arch, "-syntax", "auto", "-o", out)
	mustContain(t, read(t, out), "MOVUPS", "ADDPS", "RET")
}

func TestRunGeneratesCallableGoPackage(t *testing.T) {
	goos, arch := requireHostCompilerTarget(t)
	dir := t.TempDir()
	src := write(t, dir, "sample.c", "//go:build ignore\n\nint add(int a, int b) { return a + b; }\nlong add64(long a, long b) { return a + b; }\n")
	asmPath, goPath := filepath.Join(dir, "sample_"+arch+".s"), filepath.Join(dir, "sample.go")
	runOK(t, "-src", src, "-cc", "clang", "-arch", arch, "-syntax", "auto", "-pkg", "sample", "-o", asmPath, "-go", goPath)
	mustContain(t, read(t, goPath), "package sample", "func Add(a int32, b int32) int32", "func Add64(")
	mustContain(t, read(t, asmPath), packageAsmChecks(arch)...)
	write(t, dir, "go.mod", "module sample\n\ngo 1.26\n")
	write(t, dir, "sample_test.go", "package sample\n\nimport \"testing\"\n\nfunc TestGenerated(t *testing.T) { _ = Add(2, 3); _ = Add64(2, 3) }\n")
	goTest(t, dir, goos, arch, "-c", "./...")
}

func TestRunPackageModeForGoGenerate(t *testing.T) {
	goos, arch := requireHostCompilerTarget(t)
	dir := t.TempDir()
	writeAll(t, dir, map[string]string{
		"generate.go": "package sample\n\n//go:generate go run github.com/frankli0324/go-c2go/cmd/c2go -c sample.c\n",
		"go.mod":      "module sample\n\ngo 1.26\n",
		"sample.c": `
//go:build ignore
int add(int a, int b) { return a + b; }
long add64(long a, long b) { return a + b; }
int first(const char *buf, int buf_len) { return buf_len > 0 ? buf[0] : 0; }
unsigned char id_u8(unsigned char v) { return v; }
short id_i16(short v) { return v; }
unsigned int id_u32(unsigned int v) { return v; }
long long id_i64(long long v) { return v; }
`,
		"sample_test.go": `package sample
import "testing"
func TestGeneratedC(t *testing.T) {
	if Add(2, 3) != 5 || Add64(2, 3) != 5 || First([]byte("abc")) != 'a' ||
		IdU8(42) != 42 || IdI16(-42) != -42 || IdU32(42) != 42 || IdI64(-42) != -42 {
		t.Fatal("generated binding returned wrong value")
	}
}
`,
	})
	inDir(t, dir, func() {
		t.Setenv("GOARCH", arch)
		runOK(t, "-cc", "clang", "-syntax", "auto", "-c", "sample.c")
	})
	for _, name := range []string{"sample_c2go.go", "sample_c2go_" + arch + ".s"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected generated file %s: %v", name, err)
		}
	}
	goTest(t, dir, goos, arch, "-v", "./...")
}

func TestRunPackageModeRequiresCFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "generate.go", "package sample\n")
	inDir(t, dir, func() {
		err := run(nil)
		if err == nil || !strings.Contains(err.Error(), "requires at least one -c") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func requireHostCompilerTarget(t *testing.T) (string, string) {
	t.Helper()
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang not available")
	}
	goos, arch := runtime.GOOS, runtime.GOARCH
	if !supportsTarget(goos, arch) {
		t.Skipf("compiler target is not configured for %s/%s", goos, arch)
	}
	return goos, arch
}

func assertTranslatedArithmetic(t *testing.T, arch, text string) {
	t.Helper()
	if arch == asmconv.ArchARM64 {
		mustContainAny(t, text, "ADDW", "ADD")
		return
	}
	mustContainAny(t, text, "LEAL", "LEAQ", "MOVL")
}

func write(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func writeAll(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		write(t, dir, name, body)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

func runOK(t *testing.T, args ...string) {
	t.Helper()
	if err := run(args); err != nil {
		t.Fatalf("run() failed: %v", err)
	}
}

func inDir(t *testing.T, dir string, fn func()) {
	t.Helper()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()
	fn()
}

func goTest(t *testing.T, dir, goos, arch string, args ...string) {
	t.Helper()
	cmd := exec.Command("go", append([]string{"test"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOARCH="+arch, "GOOS="+goos)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated package should pass go test: %v\n%s", err, out)
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

func mustContainAny(t *testing.T, text string, checks ...string) {
	t.Helper()
	for _, want := range checks {
		if strings.Contains(text, want) {
			return
		}
	}
	t.Fatalf("output missing any of %v\n%s", checks, text)
}

func packageAsmChecks(arch string) []string {
	checks := []string{"TEXT c2go_add(SB), NOSPLIT|NOFRAME, $0", "CALL c2go_add(SB)"}
	if arch == asmconv.ArchARM64 {
		return append(checks, "TEXT ·Add(SB), NOSPLIT, $0-12", "MOVW a+0(FP), R0")
	}
	return append(checks, "TEXT ·Add(SB), NOSPLIT, $0-12", "MOVL a+0(FP), DI")
}
