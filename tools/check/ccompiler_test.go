package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyCCompilerOnPathNamesTheMissingCompiler(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	err := verifyCCompilerOnPath("gcc -m64")
	if err == nil {
		t.Fatal("verifyCCompilerOnPath() returned no error with no compiler on PATH")
	}
	if want := "Missing C compiler: 'gcc' is not on PATH"; !strings.Contains(err.Error(), want) {
		t.Fatalf("verifyCCompilerOnPath() = %q, want it to contain %q", err, want)
	}
}

func TestVerifyCCompilerOnPathRejectsAnEmptyCommand(t *testing.T) {
	if err := verifyCCompilerOnPath(""); err == nil {
		t.Fatal("verifyCCompilerOnPath() returned no error for an empty compiler command")
	}
}

func TestVerifyCCompilerOnPathAcceptsACompilerWithFlags(t *testing.T) {
	binDirPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDirPath, "gcc"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("An error occurred writing the fake compiler: %v", err)
	}
	t.Setenv("PATH", binDirPath)

	if err := verifyCCompilerOnPath("gcc -m64"); err != nil {
		t.Fatalf("verifyCCompilerOnPath() returned an error with the compiler on PATH: %v", err)
	}
}
