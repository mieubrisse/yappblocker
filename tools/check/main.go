// Command check runs yappblocker's full check pipeline: module tidiness, formatting, vet, lint,
// govulncheck, deadcode, tests with the race detector, and the per-package coverage gate.
//
// Run it from the repo root as `go -C tools run ./check`. It needs only `go` and `git` on the host:
// it re-executes itself under the Go toolchain pinned in src/go.mod, builds each check tool from the
// version pinned in its own modfile in this directory, and fails if any version it ran with differs
// from its pin. That is what makes CI, the pre-commit hook and an agent's shell reach the same
// verdict.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/mieubrisse/stacktrace"
)

const (
	// srcDirName is the directory holding yappblocker's own Go module, relative to the repo root.
	srcDirName = "src"

	// toolsDirName is the directory holding this runner's module and the check tools' modfiles,
	// relative to the repo root. `go -C tools run ./check` makes it the working directory.
	toolsDirName = "tools"

	// runnerPackage is this command's package, relative to the tools module, used to re-execute it.
	runnerPackage = "./check"

	// reexecGuardEnvVar marks a run that has already re-executed itself under the pinned toolchain,
	// so a toolchain that still reports the wrong version fails instead of looping.
	reexecGuardEnvVar = "YAPPBLOCKER_CHECK_REEXECED"

	// goToolchainEnvVar is the environment variable that makes the go command run one exact
	// toolchain; the go.mod `toolchain` directive alone is only a suggestion.
	goToolchainEnvVar = "GOTOOLCHAIN"

	// goWorkEnvVar is set to off so a go.work file above the repo cannot change module resolution.
	goWorkEnvVar = "GOWORK"

	// vulnDBEnvVar, when set, is the vulnerability database URL govulncheck reads instead of its
	// default, https://vuln.go.dev. The container envelope (check-in-container.sh) runs with no
	// network, so it mirrors the database into a file:// URL and points this at it. govulncheck
	// itself has only a -db flag, no environment variable.
	vulnDBEnvVar = "YAPPBLOCKER_CHECK_VULN_DB"
)

var (
	// verdictAffectingEnvVars are go settings from the caller's shell that would change what the
	// check builds or tests, and so make its verdict differ from CI's. The runner clears them.
	verdictAffectingEnvVars = []string{"GOFLAGS", "GOOS", "GOARCH", "GOEXPERIMENT"}
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ Check failed:\n%v\n", err)
		os.Exit(1)
	}
	fmt.Println("\n✓ All checks passed")
}

func run() error {
	toolsDirPath, err := os.Getwd()
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred getting the working directory")
	}
	if filepath.Base(toolsDirPath) != toolsDirName {
		return stacktrace.NewError("Expected to run from the '%v' directory, but the working directory is '%v'; run `go -C tools run ./check` from the repo root", toolsDirName, toolsDirPath)
	}
	repoRootDirPath := filepath.Dir(toolsDirPath)
	srcDirPath := filepath.Join(repoRootDirPath, srcDirName)

	if err := clearVerdictAffectingEnv(); err != nil {
		return stacktrace.Propagate(err, "An error occurred clearing the go settings that would make this run differ from CI")
	}

	pinnedGoVersion, err := readPinnedGoVersion(srcDirPath)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred reading the pinned Go toolchain from '%v'", srcDirPath)
	}

	if runtime.Version() != pinnedGoVersion {
		exitCode, err := reexecUnderPinnedToolchain(toolsDirPath, pinnedGoVersion)
		if err != nil {
			return stacktrace.Propagate(err, "An error occurred re-running the check under the pinned Go toolchain '%v'", pinnedGoVersion)
		}
		// The re-executed run has already printed its own verdict.
		os.Exit(exitCode)
	}

	// Every go command this runner starts, and every tool that shells out to go, gets the pin too.
	if err := os.Setenv(goToolchainEnvVar, pinnedGoVersion); err != nil {
		return stacktrace.Propagate(err, "An error occurred setting '%v' to '%v'", goToolchainEnvVar, pinnedGoVersion)
	}

	pinnedTools, err := buildAndVerifyPinnedTools(toolsDirPath, pinnedGoVersion)
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred verifying the toolchain and check-tool versions")
	}

	moduleDirPaths := []string{srcDirPath, toolsDirPath}

	step("Checking module tidiness")
	for _, dirPath := range moduleDirPaths {
		if err := runIn(dirPath, "go", "mod", "tidy", "-diff"); err != nil {
			return stacktrace.Propagate(err, "go.mod or go.sum in '%v' is not tidy; run: go -C %v mod tidy", dirPath, filepath.Base(dirPath))
		}
	}
	pass("Modules OK")

	step("Checking code formatting")
	if err := checkFormatting(moduleDirPaths); err != nil {
		return stacktrace.Propagate(err, "An error occurred checking code formatting")
	}
	pass("Formatting OK")

	step("Running go vet")
	for _, dirPath := range moduleDirPaths {
		if err := runIn(dirPath, "go", "vet", "./..."); err != nil {
			return stacktrace.Propagate(err, "go vet failed in '%v'", dirPath)
		}
	}
	pass("Vet OK")

	step("Running linter")
	if err := runIn(srcDirPath, pinnedTools[golangciLintToolName], "run", "./..."); err != nil {
		return stacktrace.Propagate(err, "golangci-lint failed")
	}
	pass("Lint OK")

	step("Running govulncheck")
	if err := runIn(srcDirPath, pinnedTools[govulncheckToolName], govulncheckArgs()...); err != nil {
		return stacktrace.Propagate(err, "govulncheck failed")
	}
	pass("Vulncheck OK")

	step("Running deadcode analysis")
	if err := runDeadcode(srcDirPath, pinnedTools[deadcodeToolName]); err != nil {
		return stacktrace.Propagate(err, "An error occurred running deadcode")
	}
	pass("Deadcode OK")

	step("Running tests with coverage")
	testOutput := &bytes.Buffer{}
	if err := runInTee(srcDirPath, testOutput, "go", "test", "-race", "-cover", "./..."); err != nil {
		return stacktrace.Propagate(err, "Tests failed in '%v'", srcDirPath)
	}
	if err := runIn(toolsDirPath, "go", "test", "./..."); err != nil {
		return stacktrace.Propagate(err, "Tests failed in '%v'", toolsDirPath)
	}
	pass("Tests passed")

	step(fmt.Sprintf("Checking per-package coverage (threshold: %v%%)", coverageThresholdPercent))
	if err := checkCoverage(testOutput.String()); err != nil {
		return stacktrace.Propagate(err, "The coverage gate failed")
	}
	pass("Coverage OK")

	return nil
}

// reexecUnderPinnedToolchain runs this command again under the pinned toolchain, which the go
// command downloads and verifies if the host does not already have it, and returns its exit code.
func reexecUnderPinnedToolchain(toolsDirPath string, pinnedGoVersion string) (int, error) {
	if os.Getenv(reexecGuardEnvVar) != "" {
		return 0, stacktrace.NewError("Re-executed under %v=%v, but the runner is still running Go '%v'", goToolchainEnvVar, pinnedGoVersion, runtime.Version())
	}
	fmt.Printf("Re-running under the pinned Go toolchain %v (started on %v)...\n", pinnedGoVersion, runtime.Version())

	cmd := exec.Command("go", "run", runnerPackage)
	cmd.Dir = toolsDirPath
	cmd.Env = append(
		os.Environ(),
		goToolchainEnvVar+"="+pinnedGoVersion,
		reexecGuardEnvVar+"=1",
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			return 0, stacktrace.Propagate(err, "An error occurred starting 'go run %v' under Go '%v'", runnerPackage, pinnedGoVersion)
		}
		return exitErr.ExitCode(), nil
	}
	return 0, nil
}

// govulncheckArgs returns govulncheck's arguments, reading the database from the URL in
// vulnDBEnvVar when one is set.
func govulncheckArgs() []string {
	vulnDBURL := os.Getenv(vulnDBEnvVar)
	if vulnDBURL == "" {
		return []string{"./..."}
	}
	fmt.Printf("  reading the vulnerability database from %v (%v)\n", vulnDBURL, vulnDBEnvVar)
	return []string{"-db", vulnDBURL, "./..."}
}

func clearVerdictAffectingEnv() error {
	for _, envVar := range verdictAffectingEnvVars {
		if err := os.Unsetenv(envVar); err != nil {
			return stacktrace.Propagate(err, "An error occurred unsetting '%v'", envVar)
		}
	}
	if err := os.Setenv(goWorkEnvVar, "off"); err != nil {
		return stacktrace.Propagate(err, "An error occurred setting '%v' to 'off'", goWorkEnvVar)
	}
	return nil
}

func checkFormatting(moduleDirPaths []string) error {
	goRootDirPath, err := outputIn(".", "go", "env", "GOROOT")
	if err != nil {
		return stacktrace.Propagate(err, "An error occurred getting the pinned toolchain's GOROOT")
	}
	// The gofmt shipped inside the pinned toolchain, not whichever gofmt is first on PATH.
	gofmtFilePath := filepath.Join(goRootDirPath, "bin", "gofmt")

	for _, dirPath := range moduleDirPaths {
		unformatted, err := outputIn(dirPath, gofmtFilePath, "-l", ".")
		if err != nil {
			return stacktrace.Propagate(err, "An error occurred running '%v' in '%v'", gofmtFilePath, dirPath)
		}
		if unformatted != "" {
			return stacktrace.NewError("Files need formatting in '%v':\n%v\nRun: gofmt -w .", dirPath, unformatted)
		}
	}
	return nil
}

// runDeadcode reports dead code without failing on it; it fails only when the analysis itself
// fails. Dead code becomes a hard error once the existing findings are cleaned up.
func runDeadcode(srcDirPath string, deadcodeFilePath string) error {
	output, err := outputIn(srcDirPath, deadcodeFilePath, "./...")
	if err != nil {
		return stacktrace.Propagate(err, "deadcode failed")
	}
	if output != "" {
		fmt.Printf("⚠ Dead code found (informational — will become a hard error after cleanup):\n%v\n", output)
	}
	return nil
}

func step(description string) {
	fmt.Printf("\n%v...\n", description)
}

func pass(description string) {
	fmt.Printf("✓ %v\n", description)
}

func runIn(dirPath string, name string, args ...string) error {
	return runInTee(dirPath, io.Discard, name, args...)
}

// runInTee runs a command with its combined output on this process's stdout, and also copies it
// into extraOutput.
func runInTee(dirPath string, extraOutput io.Writer, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dirPath
	// One writer for both streams: exec serializes writes only when Stdout and Stderr are the same
	// value, so two writers would race on extraOutput.
	combinedOutput := io.MultiWriter(os.Stdout, extraOutput)
	cmd.Stdout = combinedOutput
	cmd.Stderr = combinedOutput
	if err := cmd.Run(); err != nil {
		return stacktrace.Propagate(err, "An error occurred running '%v' with args '%v' in '%v'", name, args, dirPath)
	}
	return nil
}

// outputIn runs a command and returns its stdout with surrounding whitespace trimmed; its stderr
// goes to this process's stderr.
func outputIn(dirPath string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dirPath
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred running '%v' with args '%v' in '%v'", name, args, dirPath)
	}
	return string(bytes.TrimSpace(output)), nil
}
