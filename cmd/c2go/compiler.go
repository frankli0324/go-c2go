package main

import (
	"bytes"
	"fmt"
	"os"
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

type compilePlan struct {
	compiler       string
	args           []string
	trustFixedRegs []string
}

func compileC(cfg compileConfig) (string, compilePlan, error) {
	plan := buildCompilePlan(cfg)
	fmt.Fprintln(os.Stderr, "c2go:", shellCommand(plan.compiler, plan.args))
	cmd := exec.Command(plan.compiler, plan.args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", plan, fmt.Errorf("compile C with %s: %s", plan.compiler, msg)
	}
	return stdout.String(), plan, nil
}

func buildCompilePlan(cfg compileConfig) compilePlan {
	compiler := strings.TrimSpace(cfg.compiler)
	if compiler == "" {
		compiler = "clang"
	}
	return buildCompilePlanWith(compiler, cfg, isGNUCompiler(compiler))
}

func buildCompilePlanWith(compiler string, cfg compileConfig, gnuCompiler bool) compilePlan {
	plan := compilePlan{compiler: compiler}
	plan.args = []string{"-O3", "-S", "-x", "c", cfg.sourcePath, "-o", "-"}
	plan.args = append(defaultCompileFlags(), plan.args...)
	if cfg.arch == "arm64" {
		fixed := []string{"x18", "x26", "x27", "x28"}
		plan.args = append([]string{
			"-ffixed-x18", // Go asm doc: R18_PLATFORM is reserved on Apple ARM64 platforms.
			"-ffixed-x26", // Go obj/arm64: REGCTXT, the closure context register.
			"-ffixed-x27", // Go asm doc: R27 is reserved by the compiler and linker.
			"-ffixed-x28", // Go asm doc: R28 is reserved; obj/arm64 names it REGG.
		}, plan.args...)
		plan.trustFixedRegs = fixed
	}
	if cfg.arch == "amd64" && gnuCompiler {
		fixed := []string{"r12", "r13", "r14", "r15"}
		plan.args = append([]string{
			"-ffixed-r12", // Go obj/x86: REGENTRYTMP0, ABIInternal entry scratch register.
			"-ffixed-r13", // Go obj/x86: REGENTRYTMP1, ABIInternal entry scratch register.
			"-ffixed-r14", // Go obj/x86: REGG, the ABIInternal goroutine register.
			"-ffixed-r15", // Go obj/x86: REGEXT, used around external and dynlink code.
		}, plan.args...)
		plan.trustFixedRegs = fixed
	}
	if syntax := x86AsmSyntaxFlags(cfg); len(syntax) > 0 {
		plan.args = append(syntax, plan.args...)
	}
	if target := targetTriple(cfg.goos, cfg.arch); target != "" && strings.Contains(strings.ToLower(compiler), "clang") {
		plan.args = append([]string{"-target", target}, plan.args...)
	}
	plan.args = append(plan.args, cfg.extraFlags...)
	return plan
}

func shellCommand(name string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, shellQuote(name))
	for _, arg := range args {
		parts = append(parts, shellQuote(arg))
	}
	return strings.Join(parts, " ")
}

func shellQuote(arg string) string {
	if arg == "" {
		return "''"
	}
	if !strings.ContainsAny(arg, " \t\n'\"\\$`!*?[]{}();<>|&") {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
}

func compileArgs(compiler string, cfg compileConfig, gnuCompiler bool) []string {
	return buildCompilePlanWith(compiler, cfg, gnuCompiler).args
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
		"-fno-pic",
		"-fno-pie",
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
