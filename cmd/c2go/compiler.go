package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	asmconv "github.com/frankli0324/go-c2go/internal/asm"
)

type compileConfig struct {
	compiler   string
	sourcePath string
	arch       string
	goos       string
	syntax     string
	extraFlags []string
}

func compileC(cfg compileConfig) (string, error) {
	compiler := strings.TrimSpace(cfg.compiler)
	if compiler == "" {
		compiler = "clang"
	}
	cmd := exec.Command(compiler, compileArgs(compiler, cfg, isGNUCompiler(compiler))...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("compile C with %s: %s", compiler, msg)
	}
	return stdout.String(), nil
}

func compileArgs(compiler string, cfg compileConfig, gnuCompiler bool) []string {
	args := []string{"-O3", "-S", "-x", "c", cfg.sourcePath, "-o", "-"}
	args = append(defaultCompileFlags(), args...)
	if cfg.arch == "arm64" {
		args = append([]string{
			"-ffixed-x18", // Go asm doc: R18_PLATFORM is reserved on Apple ARM64 platforms.
			"-ffixed-x26", // Go obj/arm64: REGCTXT, the closure context register.
			"-ffixed-x27", // Go asm doc: R27 is reserved by the compiler and linker.
			"-ffixed-x28", // Go asm doc: R28 is reserved; obj/arm64 names it REGG.
		}, args...)
	}
	if cfg.arch == "amd64" && gnuCompiler {
		args = append([]string{
			"-ffixed-r12", // Go obj/x86: REGENTRYTMP0, ABIInternal entry scratch register.
			"-ffixed-r13", // Go obj/x86: REGENTRYTMP1, ABIInternal entry scratch register.
			"-ffixed-r14", // Go obj/x86: REGG, the ABIInternal goroutine register.
			"-ffixed-r15", // Go obj/x86: REGEXT, used around external and dynlink code.
		}, args...)
	}
	if syntax := x86AsmSyntaxFlags(cfg); len(syntax) > 0 {
		args = append(syntax, args...)
	}
	if target := targetTriple(cfg.goos, cfg.arch); target != "" && strings.Contains(strings.ToLower(compiler), "clang") {
		args = append([]string{"-target", target}, args...)
	}
	return append(args, cfg.extraFlags...)
}

func isGNUCompiler(compiler string) bool {
	version, err := exec.Command(compiler, "--version").CombinedOutput()
	if err != nil {
		return false
	}
	lower := strings.ToLower(string(version))
	return strings.Contains(lower, "gcc") && !strings.Contains(lower, "clang")
}

func defaultCompileFlags() []string {
	return []string{
		"-ffreestanding",
		"-fno-builtin",
		"-fno-stack-protector",
		"-fomit-frame-pointer",
		"-fno-asynchronous-unwind-tables",
		"-fno-unwind-tables",
	}
}

func x86AsmSyntaxFlags(cfg compileConfig) []string {
	if cfg.arch != "amd64" {
		return nil
	}
	switch asmconv.Resolve(cfg.syntax) {
	case "", asmconv.Auto, asmconv.ATT:
		return []string{"-masm=att"}
	case asmconv.Intel:
		return []string{"-masm=intel"}
	default:
		return nil
	}
}

func supportsTarget(goos, goarch string) bool {
	return targetTriple(goos, goarch) != ""
}

func targetTriple(goos, goarch string) string {
	switch goarch {
	case "arm64":
		switch goos {
		case "darwin":
			return "arm64-apple-macos"
		case "linux":
			return "aarch64-linux-gnu"
		}
	case "amd64":
		switch goos {
		case "darwin":
			return "x86_64-apple-macos"
		case "linux":
			return "x86_64-linux-gnu"
		}
	}
	return ""
}
