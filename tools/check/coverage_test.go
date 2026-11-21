package main

import (
	"slices"
	"testing"
)

func TestFindCoverageFailures(t *testing.T) {
	testOutput := `ok  	github.com/mieubrisse/yappblocker/internal/config	0.412s	coverage: 85.3% of statements
ok  	github.com/mieubrisse/yappblocker/internal/killer	(cached)	coverage: 41.0% of statements
	github.com/mieubrisse/yappblocker/cmd		coverage: 0.0% of statements
	github.com/mieubrisse/yappblocker		coverage: 0.0% of statements
?   	github.com/mieubrisse/yappblocker/internal/notify	[no test files]
ok  	github.com/mieubrisse/yappblocker/internal/schedule	0.2s	coverage: 60.0% of statements
some unrelated line mentioning 12.5%
`
	want := []string{
		"  ✗ github.com/mieubrisse/yappblocker/internal/killer: 41.0% < 60%",
		"  ✗ github.com/mieubrisse/yappblocker/internal/notify: no test files",
	}
	got := findCoverageFailures(testOutput)
	if !slices.Equal(got, want) {
		t.Fatalf("findCoverageFailures() = %q, want %q", got, want)
	}
}

func TestCheckCoveragePassesWhenEveryPackageClearsTheThreshold(t *testing.T) {
	testOutput := "ok  \tgithub.com/mieubrisse/yappblocker/internal/config\t0.4s\tcoverage: 85.3% of statements\n"
	if err := checkCoverage(testOutput); err != nil {
		t.Fatalf("checkCoverage() returned an error for passing output: %v", err)
	}
}

func TestIsExcludedFromCoverageMatchesWholePathSuffixes(t *testing.T) {
	cases := map[string]bool{
		"github.com/mieubrisse/yappblocker/cmd":             true,
		"github.com/mieubrisse/yappblocker/cmd/other":       false,
		"github.com/mieubrisse/yappblocker/internal/config": false,
	}
	for packagePath, want := range cases {
		if got := isExcludedFromCoverage(packagePath); got != want {
			t.Fatalf("isExcludedFromCoverage(%q) = %v, want %v", packagePath, got, want)
		}
	}
}
