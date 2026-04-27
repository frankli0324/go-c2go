package main

import (
	"slices"
	"testing"
)

func TestCompileArgsAvoidArm64ReservedRegisters(t *testing.T) {
	args := compileArgs("clang", compileConfig{arch: "arm64", goos: "darwin", sourcePath: "x.c"}, false)
	for _, flag := range []string{"-ffixed-x18", "-ffixed-x26", "-ffixed-x27", "-ffixed-x28"} {
		if !slices.Contains(args, flag) {
			t.Fatalf("compileArgs missing %s: %v", flag, args)
		}
	}
	for _, flag := range []string{"-ffixed-x29", "-ffixed-x30"} {
		if slices.Contains(args, flag) {
			t.Fatalf("compileArgs contains unsupported %s: %v", flag, args)
		}
	}
}

func TestCompileArgsAvoidAMD64GoReservedRegistersForGCC(t *testing.T) {
	gccArgs := compileArgs("gcc", compileConfig{arch: "amd64", goos: "linux", sourcePath: "x.c"}, true)
	for _, flag := range []string{"-ffixed-r12", "-ffixed-r13", "-ffixed-r14"} {
		if !slices.Contains(gccArgs, flag) {
			t.Fatalf("gcc compileArgs missing %s: %v", flag, gccArgs)
		}
	}

	clangArgs := compileArgs("clang", compileConfig{arch: "amd64", goos: "linux", sourcePath: "x.c"}, false)
	for _, flag := range []string{"-ffixed-r12", "-ffixed-r13", "-ffixed-r14"} {
		if slices.Contains(clangArgs, flag) {
			t.Fatalf("clang compileArgs contains GCC-only %s: %v", flag, clangArgs)
		}
	}
}
