package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	asmconv "github.com/frankli0324/go-c2go/internal/asm"
	"github.com/frankli0324/go-c2go/internal/codegen"
)

type config struct {
	compiler string
	source   string
	arch     string
	syntax   string
	output   string
	pkg      string
	goOutput string
	cFiles   multiFlag
	extraC   multiFlag
}

func run(args []string) error {
	cfg := config{}

	fs := flag.NewFlagSet("c2go", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&cfg.source, "src", "", "path to input C file")
	fs.StringVar(&cfg.compiler, "cc", "clang", "C compiler to invoke (clang/gcc)")
	fs.StringVar(&cfg.arch, "arch", "", "target architecture (defaults to host GOARCH)")
	fs.StringVar(&cfg.syntax, "syntax", "auto", "compiler asm syntax to emit and translate (auto, att, intel, plan9)")
	fs.StringVar(&cfg.output, "o", "", "output asm file path; defaults to <src>_<arch>.s")
	fs.StringVar(&cfg.pkg, "pkg", "", "Go package name for generated callable declarations; disabled when empty")
	fs.StringVar(&cfg.goOutput, "go", "", "Go declaration output path; defaults to <src>.go when -pkg is set")
	fs.Var(&cfg.cFiles, "c", "C file to generate for current package mode (repeatable)")
	fs.Var(&cfg.extraC, "cflag", "extra compiler flag (repeatable)")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s -src input.c [options]\n", fs.Name())
		fmt.Fprintln(fs.Output(), "")
		fmt.Fprintln(fs.Output(), "Compile C with clang/gcc, then rewrite compiler asm into Plan 9 syntax.")
		fmt.Fprintln(fs.Output(), "")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}
	if cfg.source == "" {
		return generatePackage(cfg)
	}
	return generate(cfg)
}

func generatePackage(cfg config) error {
	if strings.TrimSpace(cfg.output) != "" || strings.TrimSpace(cfg.goOutput) != "" {
		return fmt.Errorf("-o and -go require -src")
	}
	pkg, err := detectPackage(".")
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.pkg) == "" {
		cfg.pkg = pkg
	}
	if len(cfg.cFiles) == 0 {
		return fmt.Errorf("current package mode requires at least one -c file")
	}
	arch := resolveArch(cfg.arch)
	for _, src := range cfg.cFiles {
		base := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
		next := cfg
		next.source = src
		next.arch = arch
		next.output = base + "_c2go_" + arch + ".s"
		next.goOutput = base + "_c2go.go"
		if err := generate(next); err != nil {
			return err
		}
	}
	return nil
}

func generate(cfg config) error {
	compileCfg := buildPlan(cfg)
	asm, err := compileC(compileCfg)
	if err != nil {
		return err
	}

	rewritten, err := asmconv.Translate(asm, asmconv.Context{
		Syntax:    cfg.syntax,
		Arch:      compileCfg.arch,
		GoVersion: currentModuleGoVersion(),
	})
	var unsupported asmconv.UnsupportedError
	if err != nil && !errors.As(err, &unsupported) {
		return err
	}
	output, err := codegen.RenderAsmFile(rewritten)
	if err != nil {
		return err
	}
	if unsupported.Count > 0 {
		fmt.Fprintf(os.Stderr, "c2go: %d unsupported line(s)\n", unsupported.Count)
	}

	asmPath := asmOutputPath(cfg.output, cfg.source, compileCfg.arch)
	if pkg := strings.TrimSpace(cfg.pkg); pkg != "" {
		src, err := os.ReadFile(cfg.source)
		if err != nil {
			return err
		}
		generated, err := codegen.GenerateBinding(string(src), output, pkg, compileCfg.goos, compileCfg.arch)
		if err != nil {
			return err
		}
		arches := bindingArches(asmPath, compileCfg.arch)
		goPath := outputPath(cfg.goOutput, cfg.source, ".go")
		generated.Go = addBuildConstraint(generated.Go, arches)
		if err := writeFile(asmPath, generated.Asm); err != nil {
			return err
		}
		if err := writeFile(goPath, generated.Go); err != nil {
			return err
		}
		return writeFallbackFile(genericOutputPath(goPath), generated.Fallback, arches)
	}
	return writeFile(asmPath, output)
}

func writeFile(path, body string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(body), 0o644)
}

func buildPlan(cfg config) compileConfig {
	return compileConfig{
		compiler:   cfg.compiler,
		sourcePath: cfg.source,
		arch:       resolveArch(cfg.arch),
		goos:       resolveEnv("GOOS", runtime.GOOS),
		syntax:     cfg.syntax,
		extraFlags: append([]string(nil), cfg.extraC...),
	}
}

func outputPath(explicit, src, suffix string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	return strings.TrimSuffix(src, filepath.Ext(src)) + suffix
}

func asmOutputPath(explicit, src, arch string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	return strings.TrimSuffix(src, filepath.Ext(src)) + "_" + arch + ".s"
}

func genericOutputPath(goPath string) string {
	ext := filepath.Ext(goPath)
	if ext == "" {
		return goPath + "_generic.go"
	}
	return strings.TrimSuffix(goPath, ext) + "_generic" + ext
}

func bindingArches(asmPath, arch string) []string {
	seen := map[string]bool{arch: true}
	dir, name := filepath.Dir(asmPath), filepath.Base(asmPath)
	prefix, ok := strings.CutSuffix(name, arch+".s")
	if ok {
		if matches, err := filepath.Glob(filepath.Join(dir, prefix+"*.s")); err == nil {
			for _, match := range matches {
				base := filepath.Base(match)
				if !strings.HasPrefix(base, prefix) || !strings.HasSuffix(base, ".s") {
					continue
				}
				candidate := strings.TrimSuffix(strings.TrimPrefix(base, prefix), ".s")
				if candidate != "" {
					seen[candidate] = true
				}
			}
		}
	}
	var arches []string
	for _, candidate := range []string{asmconv.ArchAMD64, asmconv.ArchARM64} {
		if seen[candidate] {
			arches = append(arches, candidate)
		}
	}
	if len(arches) == 0 {
		return []string{arch}
	}
	return arches
}

func addBuildConstraint(src string, arches []string) string {
	if len(arches) == 0 {
		return src
	}
	return addBuildExpr(src, strings.Join(arches, " || "))
}

func addBuildExpr(src, expr string) string {
	if strings.Contains(src, "\npackage ") {
		return strings.Replace(src, "\npackage ", "\n//go:build "+expr+"\n\npackage ", 1)
	}
	return "//go:build " + expr + "\n\n" + src
}

func writeFallbackFile(path, body string, arches []string) error {
	expr := fallbackBuildExpr(arches)
	body = addBuildExpr(body, expr)
	if existing, err := os.ReadFile(path); err == nil {
		return writeFile(path, replaceBuildExpr(string(existing), expr))
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return writeFile(path, body)
}

func fallbackBuildExpr(arches []string) string {
	parts := make([]string, 0, len(arches))
	for _, arch := range arches {
		parts = append(parts, "!"+arch)
	}
	return strings.Join(parts, " && ")
}

func replaceBuildExpr(src, expr string) string {
	lines := strings.SplitAfter(src, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//go:build ") {
			lines[i] = "//go:build " + expr + "\n"
			return strings.Join(lines, "")
		}
		if strings.HasPrefix(trimmed, "package ") {
			break
		}
	}
	return addBuildExpr(src, expr)
}

func resolveArch(arch string) string {
	arch = strings.TrimSpace(strings.ToLower(arch))
	if arch != "" {
		return arch
	}
	return resolveEnv("GOARCH", runtime.GOARCH)
}

func resolveEnv(name, fallback string) string {
	if env := strings.TrimSpace(strings.ToLower(os.Getenv(name))); env != "" {
		return env
	}
	return fallback
}

func currentModuleGoVersion() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		body, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil {
			return parseGoDirective(string(body))
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func parseGoDirective(mod string) string {
	for _, line := range strings.Split(mod, "\n") {
		line = strings.TrimSpace(strings.Split(line, "//")[0])
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "go" {
			return normalizeGoVersion(fields[1])
		}
	}
	return ""
}

func normalizeGoVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || strings.HasPrefix(v, "go") {
		return v
	}
	return "go" + v
}

func detectPackage(dir string) (string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return "", err
	}
	for _, file := range files {
		name := filepath.Base(file)
		if strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, "_c2go.go") {
			continue
		}
		body, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		for _, line := range strings.Split(string(body), "\n") {
			fields := strings.Fields(line)
			if len(fields) == 2 && fields[0] == "package" {
				return fields[1], nil
			}
		}
	}
	return "", fmt.Errorf("no Go package declaration found in current directory")
}

type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}
