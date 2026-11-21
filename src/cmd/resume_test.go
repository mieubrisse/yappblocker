package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mieubrisse/yappblocker/internal/pause"
)

func TestResumeEndsAPause(t *testing.T) {
	paths := newTestFilePaths(t)
	writeActivePause(t, paths, timeInsideWindow)
	resumeTime := timeInsideWindow.Add(10 * time.Minute)
	var out bytes.Buffer

	if err := runResume(resumeTime, &out, paths.pauseStateFilePath); err != nil {
		t.Fatalf("runResume: %v", err)
	}
	if !strings.Contains(out.String(), "Blocking is back on.") {
		t.Errorf("output %q does not confirm the resume", out.String())
	}

	state, err := pause.Load(paths.pauseStateFilePath)
	if err != nil {
		t.Fatal(err)
	}
	if pause.IsPaused(state, resumeTime) {
		t.Error("still paused after resume")
	}
}

func TestResumeWithNoPauseSaysSo(t *testing.T) {
	paths := newTestFilePaths(t)
	var out bytes.Buffer

	if err := runResume(timeInsideWindow, &out, paths.pauseStateFilePath); err != nil {
		t.Fatalf("runResume: %v", err)
	}
	if !strings.Contains(out.String(), "nothing to resume") {
		t.Errorf("output %q does not say there was nothing to resume", out.String())
	}
}

func TestResumeWithADamagedStateFileFails(t *testing.T) {
	paths := newTestFilePaths(t)
	writeTestFile(t, paths.pauseStateFilePath, "{not json")

	if err := runResume(timeInsideWindow, &bytes.Buffer{}, paths.pauseStateFilePath); err == nil {
		t.Error("expected an error with a damaged pause state")
	}
}
