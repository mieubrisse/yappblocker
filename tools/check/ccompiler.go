package main

import (
	"os"
	"runtime"

	"github.com/mieubrisse/stacktrace"
)

const (
	// darwinGOOS is the one platform where the go command's race detector does not need cgo.
	darwinGOOS = "darwin"

	// cgoProbePackage is a standard-library cgo package that `go test -race` builds off macOS.
	// Building it needs both the C compiler and the C library's headers.
	cgoProbePackage = "runtime/cgo"

	cgoEnabledEnvVar = "CGO_ENABLED"
)

// verifyCCompiler fails, before any check tool is built or test is run, if this host lacks the C
// compiler that the race-detector tests need everywhere but macOS.
//
// It turns cgo on explicitly, for this and every later go command. By default the go command turns
// cgo off when it finds no C compiler, so the test step would fail with `go: -race requires cgo`,
// which does not say what is missing; with cgo on, it names the compiler it could not run. Turning
// it on also overrides a CGO_ENABLED=0 in the caller's shell, which would make the verdict differ
// from CI's.
func verifyCCompiler(dirPath string) error {
	if runtime.GOOS == darwinGOOS {
		return nil
	}
	step("Verifying the C compiler that the race detector needs")
	if err := os.Setenv(cgoEnabledEnvVar, "1"); err != nil {
		return stacktrace.Propagate(err, "An error occurred setting '%v' to '1'", cgoEnabledEnvVar)
	}
	if err := buildCgoProbe(dirPath); err != nil {
		return stacktrace.Propagate(err, "An error occurred building with the C compiler")
	}
	pass("C compiler OK")
	return nil
}

func buildCgoProbe(dirPath string) error {
	// Only the go command knows how to read CC (quoting, flags, its env file), so let it decide
	// whether the compiler works, and report the one it used.
	compilerCommand, err := outputIn(dirPath, "go", "env", "CC")
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred getting the C compiler the go command uses")
	}
	if err := runIn(dirPath, "go", "build", "-race", cgoProbePackage); err != nil {
		return stacktrace.Propagate(
			err,
			"The C compiler '%v' is missing, or cannot build what the race detector (`go test -race`) needs on %v; the go command's error is above. "+
				"Install a C compiler and the C library headers (on Debian or Ubuntu: gcc and libc6-dev), or set CC to a compiler this host has. "+
				"On a host with rootless Podman, `bash tools/check-in-container.sh` needs neither",
			compilerCommand, runtime.GOOS,
		)
	}
	return nil
}
