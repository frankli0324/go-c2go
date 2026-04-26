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
	source string
	arch   string
	syntax string
	output string
}

func run(args []string) error {
	cfg := config{}

	fs := flag.NewFlagSet("asm2go", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&cfg.source, "src", "", "path to input asm file")
	fs.StringVar(&cfg.arch, "arch", "", "target architecture metadata (defaults to host GOARCH)")
	fs.StringVar(&cfg.syntax, "syntax", "att", "input asm syntax (att, intel, plan9)")
	fs.StringVar(&cfg.output, "o", "", "output file path; defaults to <src>_<arch>.s")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s -src input.s [options]\n", fs.Name())
		fmt.Fprintln(fs.Output(), "")
		fmt.Fprintln(fs.Output(), "Rewrite x86-64 asm text into Plan 9 syntax according to docs/specs/x86-64-asm-to-plan9.md.")
		fmt.Fprintln(fs.Output(), "")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}
	if cfg.source == "" {
		fs.Usage()
		return fmt.Errorf("-src is required")
	}

	asm, err := os.ReadFile(cfg.source)
	if err != nil {
		return err
	}

	arch := resolveArch(cfg.arch)
	rewritten, err := asmconv.Translate(cfg.syntax, arch, string(asm))
	var unsupported asmconv.UnsupportedError
	if err != nil && !errors.As(err, &unsupported) {
		return err
	}
	output, err := codegen.RenderAsmFile(rewritten)
	if err != nil {
		return err
	}
	if unsupported.Count > 0 {
		fmt.Fprintf(os.Stderr, "asm2go: %d unsupported line(s)\n", unsupported.Count)
	}

	return writeFile(outputPath(cfg.output, cfg.source, arch), output)
}

func writeFile(path, body string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(body), 0o644)
}

func outputPath(explicit, src, arch string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	return strings.TrimSuffix(src, filepath.Ext(src)) + "_" + arch + ".s"
}

func resolveArch(arch string) string {
	if strings.TrimSpace(arch) != "" {
		return arch
	}
	if env := strings.TrimSpace(os.Getenv("GOARCH")); env != "" {
		return env
	}
	return runtime.GOARCH
}
