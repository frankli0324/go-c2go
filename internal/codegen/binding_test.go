package codegen

import (
	"runtime"
	"strings"
	"testing"

	asmconv "github.com/frankli0324/go-c2go/internal/asm"
)

func TestParseFunctionsAndRenderDecls(t *testing.T) {
	goos, arch := currentTarget(t)
	funcs, err := parseFunctions(`
int add(int a, int b) { return a + b; }
long add64(long a, long b) { return a + b; }
void sink(int v) { (void)v; }
int first(const unsigned char *buf, size_t buf_len) { return buf_len > 0 ? buf[0] : 0; }
char id_char(char v) { return v; }
unsigned char id_uchar(unsigned char v) { return v; }
short id_short(short v) { return v; }
unsigned short id_ushort(unsigned short v) { return v; }
unsigned int id_uint(unsigned int v) { return v; }
long long id_ll(long long v) { return v; }
unsigned long long id_ull(unsigned long long v) { return v; }
void *id_ptr(void *p) { return p; }
void *id_const_ptr(const void *p) { return (void*)p; }
size_t id_size(size_t n) { return n; }
`, goos, arch)
	if err != nil {
		t.Fatalf("parseFunctions() error = %v", err)
	}
	if len(funcs) != 14 {
		t.Fatalf("len(funcs) = %d, want 14", len(funcs))
	}
	got := renderDecls("sample", arch, funcs)
	mustContain(t, got,
		"package sample",
		"import \"unsafe\"",
		"func Add(a int32, b int32) int32",
		"func Sink(v int32)",
		"func First(buf []byte) int32",
		"func IdChar(v int8) int8",
		"func IdUchar(v uint8) uint8",
		"func IdShort(v int16) int16",
		"func IdUshort(v uint16) uint16",
		"func IdUint(v uint32) uint32",
		"func IdLl(v int64) int64",
		"func IdUll(v uint64) uint64",
		"func IdPtr(p unsafe.Pointer) unsafe.Pointer",
		"func IdConstPtr(p unsafe.Pointer) unsafe.Pointer",
		"func IdSize(n uint) uint",
	)
	add64 := funcs[1]
	wantAdd64 := "func Add64(a " + add64.Params[0].Type.GoName + ", b " + add64.Params[1].Type.GoName + ") " + add64.Return.GoName
	mustContain(t, got, wantAdd64)
	if strings.Contains(got, "//go:build") {
		t.Fatalf("declarations should not have arch build tags\n%s", got)
	}
}

func TestWrapAssemblyRenamesRawSymbolsAndAddsHostWrappers(t *testing.T) {
	goos, arch := currentTarget(t)
	funcs, err := parseFunctions(`
int add(int a, int b) { return a + b; }
long add64(long a, long b) { return a + b; }
int first(const unsigned char *buf, size_t buf_len) { return buf_len > 0 ? buf[0] : 0; }
unsigned short id_ushort(unsigned short v) { return v; }
void *id_ptr(void *p) { return p; }
`, goos, arch)
	if err != nil {
		t.Fatalf("parseFunctions() error = %v", err)
	}
	asm := textForSymbols(goos, []string{"add", "add64", "first", "id_ushort", "id_ptr"})
	got := wrapAssembly(asm, funcs, goos, arch)
	mustContain(t, got, append([]string{
		"TEXT c2go_add(SB), NOSPLIT|NOFRAME, $0",
		"TEXT c2go_add64(SB), NOSPLIT|NOFRAME, $0",
		"TEXT c2go_first(SB), NOSPLIT|NOFRAME, $0",
		"TEXT c2go_id_ushort(SB), NOSPLIT|NOFRAME, $0",
		"TEXT c2go_id_ptr(SB), NOSPLIT|NOFRAME, $0",
		"CALL c2go_add(SB)",
	}, hostWrapperChecks(arch)...)...)
	if strings.Contains(got, "TEXT "+compilerSymbol(goos, "add")+"(SB)") {
		t.Fatalf("compiler symbol should be renamed\n%s", got)
	}
}

func TestBytesReturnIsRejected(t *testing.T) {
	goos, arch := currentTarget(t)
	_, err := parseFunctions(`const char *bad(const char *buf, size_t buf_len) { return buf; }`, goos, arch)
	if err == nil {
		t.Fatal("expected error for []byte return")
	}
	if !strings.Contains(err.Error(), "[]byte returns are not supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestByteParamRequiresConstCharPointerAndIntPair(t *testing.T) {
	goos, arch := currentTarget(t)
	funcs, err := parseFunctions(`
int first(const unsigned char *buf, size_t n) { return n > 0 ? buf[0] : 0; }
int second(int prefix, const char *data, size_t data_len) { return prefix + (int)data_len; }
`, goos, arch)
	if err != nil {
		t.Fatalf("parseFunctions() error = %v", err)
	}
	if got := renderDecls("sample", arch, funcs); !strings.Contains(got, "func First(buf []byte) int32") || !strings.Contains(got, "func Second(prefix int32, data []byte) int32") {
		t.Fatalf("generated declarations do not fold const char*, int into []byte\n%s", got)
	}

	for _, src := range []string{
		`int bad(const char *buf) { return buf[0]; }`,
		`int bad(const char *buf, int n) { return n; }`,
	} {
		_, err := parseFunctions(src, goos, arch)
		if err == nil || !strings.Contains(err.Error(), "requires a following size_t length parameter") {
			t.Fatalf("parseFunctions(%q) error = %v, want []byte length error", src, err)
		}
	}
}

func TestParseFunctionsUsesMarkedSubsetOnlyWhenAnyFunctionIsMarked(t *testing.T) {
	goos, arch := currentTarget(t)
	funcs, err := parseFunctions(`
int helper(int v) { return v + 1; }
//go:c2go
int add(int a, int b) { return helper(a) + b - 1; }
//go:c2go
int sub(int a, int b) { return a - b; }
`, goos, arch)
	if err != nil {
		t.Fatalf("parseFunctions() error = %v", err)
	}
	if len(funcs) != 2 {
		t.Fatalf("len(funcs) = %d, want 2", len(funcs))
	}
	for _, fn := range funcs {
		if fn.CName == "helper" {
			t.Fatalf("unmarked helper should not get a binding")
		}
	}
}

func currentTarget(t *testing.T) (string, string) {
	t.Helper()
	arch := runtime.GOARCH
	switch arch {
	case asmconv.ArchAMD64, asmconv.ArchARM64:
		return runtime.GOOS, arch
	default:
		t.Skipf("unsupported host architecture %s", arch)
		return "", ""
	}
}

func textForSymbols(goos string, names []string) string {
	var b strings.Builder
	b.WriteString("#include \"textflag.h\"\n")
	for _, name := range names {
		b.WriteString("\nTEXT ")
		b.WriteString(compilerSymbol(goos, name))
		b.WriteString("(SB), NOSPLIT|NOFRAME, $0\n\tRET\n")
	}
	return b.String()
}

func hostWrapperChecks(arch string) []string {
	common := []string{"TEXT ·Add(SB), NOSPLIT, $0-12", "TEXT ·Add64(SB), NOSPLIT, $0-", "TEXT ·First(SB), NOSPLIT, $0-28", "TEXT ·IdUshort(SB), NOSPLIT, $0-10", "TEXT ·IdPtr(SB), NOSPLIT, $0-16"}
	if arch == asmconv.ArchARM64 {
		return append(common, "MOVW a+0(FP), R0", "MOVW b+4(FP), R1", "MOVW R0, ret+8(FP)", "MOVD buf+0(FP), R0", "MOVD buf+8(FP), R1", "MOVW R0, ret+24(FP)", "MOVHU v+0(FP), R0", "MOVH R0, ret+8(FP)", "MOVD p+0(FP), R0", "MOVD R0, ret+8(FP)")
	}
	return append(common, "MOVL a+0(FP), DI", "MOVL b+4(FP), SI", "MOVL AX, ret+8(FP)", "MOVQ buf+0(FP), DI", "MOVQ buf+8(FP), SI", "MOVL AX, ret+24(FP)", "MOVWLZX v+0(FP), DI", "MOVW AX, ret+8(FP)", "MOVQ p+0(FP), DI", "MOVQ AX, ret+8(FP)")
}

func mustContain(t *testing.T, text string, checks ...string) {
	t.Helper()
	for _, want := range checks {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q\n%s", want, text)
		}
	}
}
