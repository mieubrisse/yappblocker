package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/mieubrisse/stacktrace"
)

const (
	// darwinGOOS is the one platform where the go command's race detector does not need cgo.
	darwinGOOS = "darwin"

	// raceRuntimePackage is the package `go test -race` compiles with the C compiler.
	raceRuntimePackage = "runtime/cgo"

	installCCompilerAdvice = "Install a C compiler and the C library headers (on Debian or Ubuntu: gcc and libc6-dev), " +
		"or set CC to a compiler this host has. On a host with Podman, `bash tools/check-in-container.sh` needs neither"
)

// verifyCCompiler fails, before anything slow runs, if this host lacks the C compiler that the
// race-detector tests need everywhere but macOS. Without it the go command turns cgo off silently
// and the test step fails with `go: -race requires cgo`, which does not say what is missing.
func verifyCCompiler(dirPath string) error {
	if runtime.GOOS == darwinGOOS {
		return nil
	}
	step("Verifying the C compiler that the race detector needs")

	compilerCommand, err := outputIn(dirPath, "go", "env", "CC")
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred getting the C compiler the go command uses")
	}
	if err := verifyCCompilerOnPath(compilerCommand); err != nil {
		return err
	}
	// A compiler without the C library's headers is on PATH but cannot build the race runtime.
	if err := runIn(dirPath, "go", "build", "-race", raceRuntimePackage); err != nil {
		return stacktrace.Propagate(err, "The C compiler '%v' is on PATH but cannot build what the race detector (`go test -race`) needs; its output is above. %v", compilerCommand, installCCompilerAdvice)
	}
	fmt.Printf("  %v\n", compilerCommand)
	pass("C compiler OK")
	return nil
}

// verifyCCompilerOnPath takes a compiler command as `go env CC` prints it, flags included.
func verifyCCompilerOnPath(compilerCommand string) error {
	fields := strings.Fields(compilerCommand)
	if len(fields) == 0 {
		return stacktrace.NewError("The go command names no C compiler (`go env CC` is empty), which the race detector (`go test -race`) needs on %v. %v", runtime.GOOS, installCCompilerAdvice)
	}
	if _, err := exec.LookPath(fields[0]); err != nil {
		return stacktrace.NewError("Missing C compiler: '%v' is not on PATH, and the race detector (`go test -race`) needs it on %v. %v", fields[0], runtime.GOOS, installCCompilerAdvice)
	}
	return nil
}
