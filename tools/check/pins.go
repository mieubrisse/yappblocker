package main

import (
	"debug/buildinfo"
	"encoding/json"
	"fmt"

	"github.com/mieubrisse/stacktrace"
)

const (
	golangciLintToolName = "golangci-lint"
	govulncheckToolName  = "govulncheck"
	deadcodeToolName     = "deadcode"
)

// pinnedTool is a check tool built from source at the version pinned in its own modfile in the
// tools directory. One modfile per tool keeps each tool's dependency graph out of the project's and
// out of the other tools' — golangci-lint's documented recipe for building it with `go tool`.
type pinnedTool struct {
	name        string
	modFileName string
	modulePath  string
}

var (
	// pinnedTools are the check tools, each pinned by exact version in its modfile. Update one with
	// `go -C tools get -tool -modfile=<tool>.mod <package>@<version>`.
	pinnedTools = []*pinnedTool{
		{name: golangciLintToolName, modFileName: "golangci-lint.mod", modulePath: "github.com/golangci/golangci-lint/v2"},
		{name: govulncheckToolName, modFileName: "govulncheck.mod", modulePath: "golang.org/x/vuln"},
		{name: deadcodeToolName, modFileName: "deadcode.mod", modulePath: "golang.org/x/tools"},
	}
)

// goModJSON is the subset of `go mod edit -json` output this runner reads.
type goModJSON struct {
	Toolchain string
	Require   []struct {
		Path    string
		Version string
	}
}

// readPinnedGoVersion returns the toolchain pinned by src/go.mod's `toolchain` directive, e.g.
// "go1.27.4". That directive is the single place the Go version is pinned: CI's setup-go reads it,
// and this runner enforces it exactly through GOTOOLCHAIN.
func readPinnedGoVersion(srcDirPath string) (string, error) {
	goMod, err := readGoMod(srcDirPath, "go.mod")
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred reading go.mod in '%v'", srcDirPath)
	}
	if goMod.Toolchain == "" {
		return "", stacktrace.NewError("Expected go.mod in '%v' to pin the Go toolchain with a `toolchain` directive, but it has none", srcDirPath)
	}
	return goMod.Toolchain, nil
}

// buildAndVerifyPinnedTools builds every pinned tool, prints the Go and tool versions this run
// uses, and fails unless each matches its pin. It returns each tool's binary path by name.
func buildAndVerifyPinnedTools(toolsDirPath string, pinnedGoVersion string) (map[string]string, error) {
	step("Verifying toolchain and check-tool versions against their pins")

	goVersion, err := outputIn(toolsDirPath, "go", "env", "GOVERSION")
	if err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred getting the go command's version")
	}
	fmt.Printf("  %-14v %-10v (pinned in src/go.mod)\n", "go", goVersion)
	if goVersion != pinnedGoVersion {
		return nil, stacktrace.NewError("The go command runs Go '%v', but src/go.mod pins '%v'", goVersion, pinnedGoVersion)
	}

	toolFilePaths := map[string]string{}
	for _, tool := range pinnedTools {
		toolFilePath, err := buildAndVerifyPinnedTool(toolsDirPath, pinnedGoVersion, tool)
		if err != nil {
			return nil, stacktrace.Propagate(err, "An error occurred verifying check tool '%v'", tool.name)
		}
		toolFilePaths[tool.name] = toolFilePath
	}
	pass("Versions match their pins")
	return toolFilePaths, nil
}

func buildAndVerifyPinnedTool(toolsDirPath string, pinnedGoVersion string, tool *pinnedTool) (string, error) {
	toolMod, err := readGoMod(toolsDirPath, tool.modFileName)
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred reading modfile '%v'", tool.modFileName)
	}
	pinnedToolVersion, err := findRequiredVersion(toolMod, tool.modulePath)
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred finding the version pinned in modfile '%v'", tool.modFileName)
	}

	// `go tool -n` builds the tool (or reuses the build cache) and prints its binary path.
	toolFilePath, err := outputIn(toolsDirPath, "go", "tool", "-modfile="+tool.modFileName, "-n", tool.name)
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred building '%v' from modfile '%v'", tool.name, tool.modFileName)
	}

	// Read what the binary itself says it was built from, rather than trusting the modfile.
	info, err := buildinfo.ReadFile(toolFilePath)
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred reading the build info of '%v'", toolFilePath)
	}
	// A binary built by `go tool` records the tool's own module as its main module.
	if info.Main.Path != tool.modulePath {
		return "", stacktrace.NewError("Expected '%v' to be built from module '%v', but its build info names main module '%v'", toolFilePath, tool.modulePath, info.Main.Path)
	}
	builtToolVersion := info.Main.Version
	fmt.Printf("  %-14v %-10v built with %v (pinned in tools/%v)\n", tool.name, builtToolVersion, info.GoVersion, tool.modFileName)

	if builtToolVersion != pinnedToolVersion {
		return "", stacktrace.NewError("'%v' was built from %v version '%v', but tools/%v pins '%v'", tool.name, tool.modulePath, builtToolVersion, tool.modFileName, pinnedToolVersion)
	}
	if info.GoVersion != pinnedGoVersion {
		return "", stacktrace.NewError("'%v' was built with Go '%v', but src/go.mod pins '%v'", tool.name, info.GoVersion, pinnedGoVersion)
	}
	return toolFilePath, nil
}

func readGoMod(dirPath string, modFileName string) (*goModJSON, error) {
	output, err := outputIn(dirPath, "go", "mod", "edit", "-json", modFileName)
	if err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred reading '%v' in '%v'", modFileName, dirPath)
	}
	goMod := &goModJSON{}
	if err := json.Unmarshal([]byte(output), goMod); err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred parsing the JSON form of '%v' in '%v'", modFileName, dirPath)
	}
	return goMod, nil
}

func findRequiredVersion(goMod *goModJSON, modulePath string) (string, error) {
	for _, requirement := range goMod.Require {
		if requirement.Path == modulePath {
			return requirement.Version, nil
		}
	}
	return "", stacktrace.NewError("Expected a requirement on module '%v', but found none", modulePath)
}
