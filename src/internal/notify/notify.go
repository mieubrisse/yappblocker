// Package notify says out loud, at the moment it happens, that an app ignored its
// kill. The log already records it, but nothing watches the log: a window can pass
// with every app still on screen while the only evidence sits in a file that gets
// opened days later, once something already feels wrong.
//
// Notifications are throttled to one per episode. An app that goes on ignoring its
// kill every two minutes through a nine-hour window is worth saying once, not two
// hundred and seventy times — and a channel that cries wolf that often is a channel
// that gets ignored, which would put the tool right back where it started. A second
// notification about the same app only arrives after it has been seen gone and then
// starts ignoring a kill again.
package notify

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// notificationTitle is what the notification is attributed to on screen. It is the
// tool's name rather than a description of the problem, because the problem goes in
// the body and the title is how Kevin knows which of his tools is talking.
const notificationTitle = "yappblocker"

// CommandRunner abstracts command execution for testability.
type CommandRunner interface {
	Run(args []string) (stdout string, err error)
}

// State is the record of which apps have already been reported, persisted between
// runs so that a long episode produces one notification rather than hundreds.
type State struct {
	// ReportedAppNames holds the apps currently known to be ignoring their kill.
	ReportedAppNames []string `json:"reportedAppNames"`
}

// AppNamesNeedingNotification returns the apps that are ignoring their kill and
// have not been reported yet. Order is stable so that a notification naming
// several apps reads the same way twice.
func AppNamesNeedingNotification(state *State, ignoredAppNames []string) []string {
	alreadyReported := make(map[string]bool)
	if state != nil {
		for _, name := range state.ReportedAppNames {
			alreadyReported[name] = true
		}
	}

	var needingNotification []string
	for _, name := range ignoredAppNames {
		if !alreadyReported[name] {
			needingNotification = append(needingNotification, name)
		}
	}

	sort.Strings(needingNotification)
	return needingNotification
}

// StateAfterRun is the state to persist once a run is over. It is exactly the set
// of apps ignoring their kill right now, so an app that finally died is forgotten
// and will be reported again if it starts ignoring a kill in some later window.
func StateAfterRun(ignoredAppNames []string) *State {
	sorted := make([]string, len(ignoredAppNames))
	copy(sorted, ignoredAppNames)
	sort.Strings(sorted)

	return &State{ReportedAppNames: sorted}
}

// Send raises one notification naming every app passed to it, rather than one per
// app: several apps surviving the same window is one situation, and splitting it
// across notifications only makes it easier to dismiss without reading.
func Send(runner CommandRunner, appNames []string) error {
	if len(appNames) == 0 {
		return nil
	}

	output, err := runner.Run(buildNotifyArgs(appNames))
	if err != nil {
		return fmt.Errorf("failed to raise a notification about %v: %w: %s", appNames, err, strings.Join(strings.Fields(output), " "))
	}

	return nil
}

// buildNotifyArgs composes the osascript call. The message says the quit was
// refused rather than that it failed, because the kill command itself reported
// success — the whole reason this notification exists.
func buildNotifyArgs(appNames []string) []string {
	message := fmt.Sprintf("Still running after being told to quit: %s", strings.Join(appNames, ", "))
	script := fmt.Sprintf("display notification %q with title %q", message, notificationTitle)

	return []string{"osascript", "-e", script}
}

// Load reads the notification state, returning nil when there is no state file,
// which is the ordinary case of nothing having been reported yet. A file that
// cannot be read or parsed is an error rather than a silent "nothing reported":
// treating it as empty would turn a damaged file into a notification every two
// minutes for the rest of the window.
func Load(stateFilePath string) (*State, error) {
	data, err := os.ReadFile(stateFilePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	// Wrapped plainly rather than with a stacktrace: this is logged on every
	// scheduled run, so the message has to stay to one readable line.
	if err != nil {
		return nil, fmt.Errorf("failed to read notification state file %q: %w", stateFilePath, err)
	}

	state := &State{}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("failed to parse notification state file %q: %w", stateFilePath, err)
	}

	return state, nil
}

// Save writes the notification state, creating its directory if needed.
func Save(stateFilePath string, state *State) error {
	dirPath := filepath.Dir(stateFilePath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create notification state directory %q: %w", dirPath, err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode notification state: %w", err)
	}

	if err := os.WriteFile(stateFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write notification state file %q: %w", stateFilePath, err)
	}

	return nil
}
