// Package pause records a temporary, bounded suspension of blocking.
//
// The rules exist to keep a pause exceptional rather than routine. Taking one opens
// a three-hour window during which further pauses stay allowed, and a twenty-four
// hour window after which pausing becomes possible again. The three hours are wall
// clock from the first pause rather than a budget of paused minutes, so taking
// several short pauses strictly loses time against taking one long one — there is
// nothing to be gained by splitting them up, and no way to accumulate more than
// three hours in a day.
package pause

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// pauseAllowanceWindow is how long after the first pause further pauses remain
	// permitted, and therefore also the longest any single pause can run.
	pauseAllowanceWindow = 3 * time.Hour
	// pauseCooldownWindow is how long after the first pause it takes for a fresh
	// allowance to begin.
	pauseCooldownWindow = 24 * time.Hour
)

// State is the record of the current pause window, persisted between runs.
type State struct {
	// WindowStart is when the first pause of this window was taken. Both the
	// three-hour allowance and the twenty-four hour cooldown are measured from it.
	WindowStart time.Time `json:"windowStart"`
	// PausedUntil is when blocking resumes.
	PausedUntil time.Time `json:"pausedUntil"`
}

// AllowanceRemaining is the longest pause that may be taken at the given moment.
// Zero means pausing is not allowed, because the three-hour window has closed and
// its cooldown has not yet expired.
func AllowanceRemaining(state *State, now time.Time) time.Duration {
	if !isWithinCooldownWindow(state, now) {
		return pauseAllowanceWindow
	}

	remaining := state.WindowStart.Add(pauseAllowanceWindow).Sub(now)
	if remaining <= 0 {
		return 0
	}

	// A window start in the future means the clock moved or the state file was
	// edited; neither should be able to buy more than the stated maximum.
	if remaining > pauseAllowanceWindow {
		return pauseAllowanceWindow
	}

	return remaining
}

// Begin returns the state that results from starting a pause of the requested
// length. The caller is expected to have consulted AllowanceRemaining first so that
// it can explain a refusal in its own words; the checks here are a backstop.
func Begin(state *State, requestedDuration time.Duration, now time.Time) (*State, error) {
	if requestedDuration <= 0 {
		return nil, fmt.Errorf("a pause has to be longer than zero, got %v", requestedDuration)
	}

	allowance := AllowanceRemaining(state, now)
	if allowance <= 0 {
		return nil, errors.New("pausing is not allowed right now")
	}
	if requestedDuration > allowance {
		return nil, fmt.Errorf("a pause of %v is longer than the %v still allowed", requestedDuration, allowance)
	}

	// Staying inside an open window keeps its original start, so the three hours
	// cannot be restarted by pausing again.
	windowStart := now
	if isWithinCooldownWindow(state, now) {
		windowStart = state.WindowStart
	}

	return &State{
		WindowStart: windowStart,
		PausedUntil: now.Add(requestedDuration),
	}, nil
}

// CooldownEndsAt is when pausing becomes possible again after the current window's
// allowance has been used up. It returns the zero time when there is no window,
// because then there is nothing to wait for.
func CooldownEndsAt(state *State) time.Time {
	if state == nil {
		return time.Time{}
	}
	return state.WindowStart.Add(pauseCooldownWindow)
}

// IsPaused reports whether blocking is suspended at the given moment.
func IsPaused(state *State, now time.Time) bool {
	return state != nil && now.Before(state.PausedUntil)
}

// Resume ends the current pause immediately. The window start is left alone, so
// stopping early gives back no allowance.
func Resume(state *State, now time.Time) *State {
	if state == nil {
		return nil
	}

	return &State{
		WindowStart: state.WindowStart,
		PausedUntil: now,
	}
}

// Load reads the pause state, returning nil when there is no state file at all,
// which is the ordinary case of never having paused. A file that exists but cannot
// be read or parsed is an error rather than a silent "not paused", so that damaging
// it cannot quietly hand back a fresh allowance.
func Load(stateFilePath string) (*State, error) {
	data, err := os.ReadFile(stateFilePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	// Wrapped plainly rather than with a stacktrace: a damaged state file is logged
	// on every scheduled run, so these messages have to stay to one readable line.
	if err != nil {
		return nil, fmt.Errorf("failed to read pause state file %q: %w", stateFilePath, err)
	}

	state := &State{}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("failed to parse pause state file %q: %w", stateFilePath, err)
	}

	return state, nil
}

// Save writes the pause state, creating its directory if needed.
func Save(stateFilePath string, state *State) error {
	dirPath := filepath.Dir(stateFilePath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create pause state directory %q: %w", dirPath, err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode pause state: %w", err)
	}

	if err := os.WriteFile(stateFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write pause state file %q: %w", stateFilePath, err)
	}

	return nil
}

// isWithinCooldownWindow reports whether a previous pause's twenty-four hour window
// is still running, which is what makes its start date the one that counts.
func isWithinCooldownWindow(state *State, now time.Time) bool {
	return state != nil && now.Before(state.WindowStart.Add(pauseCooldownWindow))
}
