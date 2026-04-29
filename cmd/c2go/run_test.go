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
	src := write(t, dir, "sample.c", "//go:build ignore\n\n//go:c2go\nint add(int a, int b) { return a + b; }\n//go:c2go\nlong add64(long a, long b) { return a + b; }\n")
	asmPath, goPath := filepath.Join(dir, "sample_"+arch+".s"), filepath.Join(dir, "sample.go")
	runOK(t, "-src", src, "-cc", "clang", "-arch", arch, "-syntax", "auto", "-pkg", "sample", "-o", asmPath, "-go", goPath)
	mustContain(t, read(t, goPath), "//go:build "+arch, "package sample", "func Add(a int32, b int32) int32", "func Add64(")
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
#include <stddef.h>
//go:c2go
int add(int a, int b) { return a + b; }
//go:c2go
long add64(long a, long b) { return a + b; }
//go:c2go
int first(const unsigned char *buf, size_t buf_len) { return buf_len > 0 ? buf[0] : 0; }
//go:c2go func Strlen1(s string) int32
int strlen1(const char *s, size_t s_len) { return (int)s_len; }
//go:c2go
unsigned char id_u8(unsigned char v) { return v; }
//go:c2go
short id_i16(short v) { return v; }
//go:c2go
unsigned int id_u32(unsigned int v) { return v; }
//go:c2go
long long id_i64(long long v) { return v; }
`,
		"sample_test.go": `package sample
import "testing"
func TestGeneratedC(t *testing.T) {
	if Add(2, 3) != 5 || Add64(2, 3) != 5 || First([]byte("abc")) != 'a' ||
		Strlen1("abcd") != 4 || IdU8(42) != 42 || IdI16(-42) != -42 || IdU32(42) != 42 || IdI64(-42) != -42 {
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
	mustContain(t, read(t, filepath.Join(dir, "sample_c2go.go")), "//go:build "+arch)
	mustContain(t, read(t, filepath.Join(dir, "sample_c2go_generic.go")),
		"//go:build !"+arch,
		"func Add(a int32, b int32) int32 {",
		"func First(buf []byte) int32 {",
		"func Strlen1(s string) int32 {",
	)
	goTest(t, dir, goos, arch, "-v", "./...")
	goTest(t, dir, "linux", "386", "-c", "./...")
}

func TestRunARM64RealCMemoryForms(t *testing.T) {
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang not available")
	}
	if !supportsTarget(runtime.GOOS, asmconv.ArchARM64) {
		t.Skipf("clang arm64 target is not configured for %s", runtime.GOOS)
	}
	dir := t.TempDir()
	writeAll(t, dir, map[string]string{
		"go.mod":      "module sample\n\ngo 1.26\n",
		"generate.go": "package sample\n",
		"sample.c": `//go:build ignore
#include <stddef.h>
#include <stdint.h>

//go:c2go
int signed_first(const unsigned char *buf, size_t buf_len) {
	if (buf_len == 0) return 0;
	return (int)(int8_t)buf[0];
}

//go:c2go
unsigned long long memmix(const unsigned char *buf, size_t buf_len) {
	if (buf_len < 12) return 0;
	const volatile unsigned char *p = buf;
	int64_t s8 = (int8_t)p[0];
	uint32_t u32 = *(const volatile uint32_t *)(buf + 4);
	uint64_t sum = (uint64_t)s8 + (uint64_t)u32;
	for (size_t i = 8; i < buf_len; i++) sum += p[i];
	return sum;
}
`,
	})
	inDir(t, dir, func() {
		runOK(t, "-cc", "clang", "-syntax", "auto", "-arch", asmconv.ArchARM64, "-c", "sample.c")
	})
	asm := read(t, filepath.Join(dir, "sample_c2go_"+asmconv.ArchARM64+".s"))
	mustContain(t, asm, "TEXT ·Memmix(SB)", "TEXT ·SignedFirst(SB)", "MOVB (R0), R0", "MOVWU")
	mustContainAny(t, asm, ".P", "ADD $1")
	mustNotContain(t, asm, "// UNSUPPORTED")
	goTest(t, dir, runtime.GOOS, asmconv.ArchARM64, "-c", "./...")
}

func TestRunPackageModeMergesGeneratedArchTags(t *testing.T) {
	goos, _ := requireHostCompilerTarget(t)
	dir := t.TempDir()
	writeAll(t, dir, map[string]string{
		"go.mod":      "module sample\n\ngo 1.26\n",
		"generate.go": "package sample\n",
		"sample.c": `//go:build ignore

//go:c2go
int add(int a, int b) { return a + b; }
`,
	})
	inDir(t, dir, func() {
		runOK(t, "-cc", "clang", "-syntax", "auto", "-arch", asmconv.ArchAMD64, "-c", "sample.c")
		mustContain(t, read(t, "sample_c2go.go"), "//go:build amd64")
		mustContain(t, read(t, "sample_c2go_generic.go"), "//go:build !amd64")
		runOK(t, "-cc", "clang", "-syntax", "auto", "-arch", asmconv.ArchARM64, "-c", "sample.c")
		mustContain(t, read(t, "sample_c2go.go"), "//go:build amd64 || arm64")
		mustContain(t, read(t, "sample_c2go_generic.go"), "//go:build !amd64 && !arm64")
	})
	goTest(t, dir, goos, asmconv.ArchAMD64, "-c", "./...")
	goTest(t, dir, goos, asmconv.ArchARM64, "-c", "./...")
}

func TestRunReadsGoModFromCurrentDirectory(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	if err := os.Mkdir(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "go.mod", "module sample\n\ngo 1.21\n")
	src := write(t, srcDir, "sample.c", "int add(int a, int b) { return a + b; }\n")
	out := filepath.Join(dir, "sample.s")
	inDir(t, dir, func() {
		runOK(t, "-src", src, "-cc", "clang", "-arch", asmconv.ArchAMD64, "-syntax", "auto", "-o", out)
	})
	text := read(t, out)
	mustContain(t, text, "// .p2align")
	mustNotContain(t, text, "PCALIGN")
}

func TestRunKeepsPCALIGNForNewGoMod(t *testing.T) {
	_, arch := requireHostCompilerTarget(t)
	if arch != asmconv.ArchAMD64 {
		t.Skipf("clang amd64 p2align check requires amd64 output, got %s", arch)
	}
	dir := t.TempDir()
	write(t, dir, "go.mod", "module sample\n\ngo 1.26\n")
	src := write(t, dir, "sample.c", "int add(int a, int b) { return a + b; }\n")
	out := filepath.Join(dir, "sample.s")
	inDir(t, dir, func() {
		runOK(t, "-src", src, "-cc", "clang", "-arch", asmconv.ArchAMD64, "-syntax", "auto", "-o", out)
	})
	mustContain(t, read(t, out), "PCALIGN")
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
	if arch == asmconv.ArchARM64 {
		return []string{"TEXT ·Add(SB), NOSPLIT, $0-12", "MOVW a+0(FP), R0"}
	}
	return []string{"TEXT ·Add(SB), NOSPLIT, $0-12", "MOVL a+0(FP), DI"}
}
