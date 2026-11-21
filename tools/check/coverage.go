package main

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/mieubrisse/stacktrace"
)

const (
	// coverageThresholdPercent is the minimum per-package test coverage. Packages below it fail
	// the check. Raise over time as coverage improves.
	coverageThresholdPercent = 60.0

	// projectModulePath prefixes every yappblocker package in `go test` output.
	projectModulePath = "github.com/mieubrisse/yappblocker"

	noTestFilesMarker = "[no test files]"
)

var (
	// coverageExcludedPackagePatterns are packages excluded from coverage enforcement. Entry-point
	// and code-generation packages that are not meaningfully unit-testable belong here.
	coverageExcludedPackagePatterns = []*regexp.Regexp{
		regexp.MustCompile(`github.com/mieubrisse/yappblocker$`),
		regexp.MustCompile(`/cmd$`),
		regexp.MustCompile(`/cmd/version$`),
		regexp.MustCompile(`/internal/buildinfo$`),
		regexp.MustCompile(`/internal/launchd$`),
	}

	coveragePercentPattern = regexp.MustCompile(`([0-9]+\.[0-9]+)%`)
)

// checkCoverage reads the output of `go test -cover ./...` and fails if any package that is not
// excluded has no test files or covers less than the threshold.
func checkCoverage(testOutput string) error {
	failures := findCoverageFailures(testOutput)
	if len(failures) > 0 {
		return stacktrace.NewError("Some packages are below the %v%% coverage threshold:\n%v", coverageThresholdPercent, strings.Join(failures, "\n"))
	}
	return nil
}

func findCoverageFailures(testOutput string) []string {
	failures := []string{}
	for _, line := range strings.Split(testOutput, "\n") {
		packagePath := findProjectPackagePath(line)
		if packagePath == "" || isExcludedFromCoverage(packagePath) {
			continue
		}
		if strings.Contains(line, noTestFilesMarker) {
			failures = append(failures, "  ✗ "+packagePath+": no test files")
			continue
		}
		match := coveragePercentPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		percent, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			failures = append(failures, "  ✗ "+packagePath+": unparseable coverage '"+match[1]+"'")
			continue
		}
		if percent < coverageThresholdPercent {
			failures = append(failures, "  ✗ "+packagePath+": "+match[1]+"% < "+strconv.FormatFloat(coverageThresholdPercent, 'f', -1, 64)+"%")
		}
	}
	return failures
}

func findProjectPackagePath(line string) string {
	for _, field := range strings.Fields(line) {
		if strings.HasPrefix(field, projectModulePath) {
			return field
		}
	}
	return ""
}

func isExcludedFromCoverage(packagePath string) bool {
	for _, pattern := range coverageExcludedPackagePatterns {
		if pattern.MatchString(packagePath) {
			return true
		}
	}
	return false
}
