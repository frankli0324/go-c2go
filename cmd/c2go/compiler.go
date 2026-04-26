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
	cmd := exec.Command(compiler, compileArgs(compiler, cfg)...)
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

func compileArgs(compiler string, cfg compileConfig) []string {
	args := []string{"-O3", "-S", "-x", "c", cfg.sourcePath, "-o", "-"}
	args = append(defaultCompileFlags(), args...)
	if cfg.arch == "arm64" {
		args = append([]string{"-ffixed-x18"}, args...)
	}
	if syntax := x86AsmSyntaxFlags(cfg); len(syntax) > 0 {
		args = append(syntax, args...)
	}
	if target := targetTriple(cfg.goos, cfg.arch); target != "" && strings.Contains(strings.ToLower(compiler), "clang") {
		args = append([]string{"-target", target}, args...)
	}
	return append(args, cfg.extraFlags...)
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
