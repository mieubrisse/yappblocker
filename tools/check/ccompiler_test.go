package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildCgoProbeNamesTheMissingCompiler(t *testing.T) {
	missingCompilerFilePath := filepath.Join(t.TempDir(), "no-such-compiler")
	t.Setenv("CC", missingCompilerFilePath)
	t.Setenv(cgoEnabledEnvVar, "1")

	err := buildCgoProbe(".")
	if err == nil {
		t.Fatal("buildCgoProbe() returned no error with no C compiler")
	}
	if want := "The C compiler '" + missingCompilerFilePath + "' is missing"; !strings.Contains(err.Error(), want) {
		t.Fatalf("buildCgoProbe() = %q, want it to contain %q", err, want)
	}
}
