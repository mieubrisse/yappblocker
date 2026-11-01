package pause

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

var windowStart = time.Date(2026, 3, 25, 20, 30, 0, 0, time.Local)

func TestAllowanceRemaining(t *testing.T) {
	tests := []struct {
		name  string
		state *State
		now   time.Time
		want  time.Duration
	}{
		{
			name:  "never paused before, the full allowance is available",
			state: nil,
			now:   windowStart,
			want:  3 * time.Hour,
		},
		{
			name:  "at the very start of a window the full allowance is available",
			state: &State{WindowStart: windowStart, PausedUntil: windowStart},
			now:   windowStart,
			want:  3 * time.Hour,
		},
		{
			name:  "an hour into the window, two hours of wall clock remain",
			state: &State{WindowStart: windowStart, PausedUntil: windowStart.Add(30 * time.Minute)},
			now:   windowStart.Add(time.Hour),
			want:  2 * time.Hour,
		},
		{
			name:  "time spent paused does not come back when the pause ended early",
			state: &State{WindowStart: windowStart, PausedUntil: windowStart.Add(10 * time.Minute)},
			now:   windowStart.Add(2 * time.Hour),
			want:  time.Hour,
		},
		{
			name:  "exactly three hours in, the allowance is gone",
			state: &State{WindowStart: windowStart, PausedUntil: windowStart.Add(time.Hour)},
			now:   windowStart.Add(3 * time.Hour),
			want:  0,
		},
		{
			name:  "past three hours, pausing is in cooldown",
			state: &State{WindowStart: windowStart, PausedUntil: windowStart.Add(time.Hour)},
			now:   windowStart.Add(5 * time.Hour),
			want:  0,
		},
		{
			name:  "one minute before the 24h window closes, still in cooldown",
			state: &State{WindowStart: windowStart, PausedUntil: windowStart.Add(time.Hour)},
			now:   windowStart.Add(24*time.Hour - time.Minute),
			want:  0,
		},
		{
			name:  "exactly 24h on, a fresh allowance begins",
			state: &State{WindowStart: windowStart, PausedUntil: windowStart.Add(time.Hour)},
			now:   windowStart.Add(24 * time.Hour),
			want:  3 * time.Hour,
		},
		{
			name:  "long after the 24h window, the full allowance is available",
			state: &State{WindowStart: windowStart, PausedUntil: windowStart.Add(time.Hour)},
			now:   windowStart.Add(40 * time.Hour),
			want:  3 * time.Hour,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := AllowanceRemaining(test.state, test.now); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestBeginStartsAndExtendsPausesWithinTheAllowance(t *testing.T) {
	tests := []struct {
		name            string
		state           *State
		requested       time.Duration
		now             time.Time
		wantWindowStart time.Time
		wantPausedUntil time.Time
	}{
		{
			name:            "a first pause opens the window at the moment it is taken",
			state:           nil,
			requested:       90 * time.Minute,
			now:             windowStart,
			wantWindowStart: windowStart,
			wantPausedUntil: windowStart.Add(90 * time.Minute),
		},
		{
			name:            "a first pause may use the whole three hours",
			state:           nil,
			requested:       3 * time.Hour,
			now:             windowStart,
			wantWindowStart: windowStart,
			wantPausedUntil: windowStart.Add(3 * time.Hour),
		},
		{
			name:            "a second pause keeps the original window open",
			state:           &State{WindowStart: windowStart, PausedUntil: windowStart.Add(30 * time.Minute)},
			requested:       30 * time.Minute,
			now:             windowStart.Add(time.Hour),
			wantWindowStart: windowStart,
			wantPausedUntil: windowStart.Add(90 * time.Minute),
		},
		{
			name:            "a second pause may run right up to the end of the three hours",
			state:           &State{WindowStart: windowStart, PausedUntil: windowStart.Add(30 * time.Minute)},
			requested:       2 * time.Hour,
			now:             windowStart.Add(time.Hour),
			wantWindowStart: windowStart,
			wantPausedUntil: windowStart.Add(3 * time.Hour),
		},
		{
			name:            "once the 24h window has passed, a pause opens a new one",
			state:           &State{WindowStart: windowStart, PausedUntil: windowStart.Add(time.Hour)},
			requested:       2 * time.Hour,
			now:             windowStart.Add(25 * time.Hour),
			wantWindowStart: windowStart.Add(25 * time.Hour),
			wantPausedUntil: windowStart.Add(27 * time.Hour),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			newState, err := Begin(test.state, test.requested, test.now)
			if err != nil {
				t.Fatalf("expected the pause to be allowed, got error: %v", err)
			}
			if !newState.WindowStart.Equal(test.wantWindowStart) {
				t.Errorf("window start: got %v, want %v", newState.WindowStart, test.wantWindowStart)
			}
			if !newState.PausedUntil.Equal(test.wantPausedUntil) {
				t.Errorf("paused until: got %v, want %v", newState.PausedUntil, test.wantPausedUntil)
			}
		})
	}
}

func TestBeginRefusesPausesOutsideTheAllowance(t *testing.T) {
	tests := []struct {
		name      string
		state     *State
		requested time.Duration
		now       time.Time
	}{
		{
			name:      "during the cooldown that follows the three hour window",
			state:     &State{WindowStart: windowStart, PausedUntil: windowStart.Add(time.Hour)},
			requested: time.Minute,
			now:       windowStart.Add(6 * time.Hour),
		},
		{
			name:      "a zero length pause",
			state:     nil,
			requested: 0,
			now:       windowStart,
		},
		{
			name:      "a negative length pause",
			state:     nil,
			requested: -time.Hour,
			now:       windowStart,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			newState, err := Begin(test.state, test.requested, test.now)
			if err == nil {
				t.Fatalf("expected the pause to be refused, but got state %+v", newState)
			}
		})
	}
}

func TestIsPaused(t *testing.T) {
	tests := []struct {
		name  string
		state *State
		now   time.Time
		want  bool
	}{
		{
			name:  "no state means blocking is on",
			state: nil,
			now:   windowStart,
			want:  false,
		},
		{
			name:  "inside the pause",
			state: &State{WindowStart: windowStart, PausedUntil: windowStart.Add(time.Hour)},
			now:   windowStart.Add(30 * time.Minute),
			want:  true,
		},
		{
			name:  "at the exact moment the pause expires, blocking is back on",
			state: &State{WindowStart: windowStart, PausedUntil: windowStart.Add(time.Hour)},
			now:   windowStart.Add(time.Hour),
			want:  false,
		},
		{
			name:  "after the pause expired",
			state: &State{WindowStart: windowStart, PausedUntil: windowStart.Add(time.Hour)},
			now:   windowStart.Add(2 * time.Hour),
			want:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsPaused(test.state, test.now); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestResumeEndsThePauseWithoutRefundingTheAllowance(t *testing.T) {
	state := &State{WindowStart: windowStart, PausedUntil: windowStart.Add(3 * time.Hour)}
	now := windowStart.Add(20 * time.Minute)

	resumed := Resume(state, now)

	if IsPaused(resumed, now) {
		t.Error("expected blocking to be back on immediately after resuming")
	}
	if !resumed.WindowStart.Equal(windowStart) {
		t.Errorf("window start: got %v, want it left alone at %v", resumed.WindowStart, windowStart)
	}
	// Resuming early must not hand back time: the three hours are wall clock from
	// the first pause, which is what makes re-pausing pointless rather than useful.
	wantRemaining := 3*time.Hour - 20*time.Minute
	if got := AllowanceRemaining(resumed, now); got != wantRemaining {
		t.Errorf("allowance after resuming: got %v, want %v", got, wantRemaining)
	}
}

func TestSaveThenLoadRoundTripsTheState(t *testing.T) {
	stateFilePath := filepath.Join(t.TempDir(), "nested", "pause.json")
	saved := &State{WindowStart: windowStart, PausedUntil: windowStart.Add(90 * time.Minute)}

	if err := Save(stateFilePath, saved); err != nil {
		t.Fatalf("saving failed: %v", err)
	}

	loaded, err := Load(stateFilePath)
	if err != nil {
		t.Fatalf("loading failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected the saved state back, got nil")
	}
	if !loaded.WindowStart.Equal(saved.WindowStart) {
		t.Errorf("window start: got %v, want %v", loaded.WindowStart, saved.WindowStart)
	}
	if !loaded.PausedUntil.Equal(saved.PausedUntil) {
		t.Errorf("paused until: got %v, want %v", loaded.PausedUntil, saved.PausedUntil)
	}
}

func TestLoadTreatsAMissingFileAsNeverPaused(t *testing.T) {
	stateFilePath := filepath.Join(t.TempDir(), "pause.json")

	loaded, err := Load(stateFilePath)
	if err != nil {
		t.Fatalf("a missing state file should not be an error, got: %v", err)
	}
	if loaded != nil {
		t.Errorf("expected nil state, got %+v", loaded)
	}
}

// A damaged state file must not be silently treated as "no pause in progress",
// because that would hand back a fresh allowance to anyone who corrupts it. The
// caller decides what to do, but it has to be told.
func TestLoadReportsAnUnreadableFileRatherThanIgnoringIt(t *testing.T) {
	stateFilePath := filepath.Join(t.TempDir(), "pause.json")
	if err := os.WriteFile(stateFilePath, []byte("this is not json"), 0644); err != nil {
		t.Fatalf("could not write the corrupt file: %v", err)
	}

	if _, err := Load(stateFilePath); err == nil {
		t.Error("expected a corrupt state file to be reported as an error")
	}
}

// A window start in the future can only come from the clock moving or the state file
// being edited by hand. Neither should buy more than the three-hour maximum.
func TestAllowanceRemainingNeverExceedsTheMaximumForAFutureWindowStart(t *testing.T) {
	now := windowStart
	future := &State{WindowStart: now.Add(5 * time.Hour), PausedUntil: now}

	if got := AllowanceRemaining(future, now); got != 3*time.Hour {
		t.Errorf("got %v, want the 3h maximum", got)
	}
}

func TestCooldownEndsAt(t *testing.T) {
	tests := []struct {
		name  string
		state *State
		want  time.Time
	}{
		{
			name:  "24h after the window opened",
			state: &State{WindowStart: windowStart, PausedUntil: windowStart.Add(time.Hour)},
			want:  windowStart.Add(24 * time.Hour),
		},
		{
			name:  "no state means there is nothing to wait for",
			state: nil,
			want:  time.Time{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CooldownEndsAt(test.state); !got.Equal(test.want) {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

// Moving the system clock backwards must not buy back the time it jumped. The leak
// this guards against was not a single over-long pause but a pair of them: a pause
// taken before the stored window start, granted a full three hours, followed by
// another one using the remainder of a window that still closed three hours after
// that stored start.
func TestChainedPauseAfterBackwardClockJumpStaysUnderTheMaximum(t *testing.T) {
	jump := 10 * time.Minute
	clockMovedBackTo := windowStart.Add(-jump)

	state := &State{WindowStart: windowStart, PausedUntil: clockMovedBackTo}

	first, err := Begin(state, 3*time.Hour, clockMovedBackTo)
	if err != nil {
		t.Fatalf("expected the first pause to be allowed, got: %v", err)
	}

	second, err := Begin(first, jump, first.PausedUntil)
	if err != nil {
		// Refusing the second pause outright is also a correct outcome here.
		return
	}

	totalPaused := first.PausedUntil.Sub(clockMovedBackTo) +
		second.PausedUntil.Sub(first.PausedUntil)
	if totalPaused > 3*time.Hour {
		t.Errorf("accumulated %v of pause in one window, want at most 3h", totalPaused)
	}
}

// A request reaching past the end of the window is trimmed to it rather than
// refused, because the few seconds spent typing the confirmation phrase would
// otherwise make the largest advertised pause the one pause nobody can take.
func TestBeginTrimsARequestThatOvershootsTheWindowEnd(t *testing.T) {
	tests := []struct {
		name            string
		state           *State
		requested       time.Duration
		now             time.Time
		wantPausedUntil time.Time
	}{
		{
			name:            "overshooting by the seconds spent typing",
			state:           &State{WindowStart: windowStart, PausedUntil: windowStart},
			requested:       2 * time.Hour,
			now:             windowStart.Add(time.Hour + 15*time.Second),
			wantPausedUntil: windowStart.Add(3 * time.Hour),
		},
		{
			name:            "a first pause asking past the maximum",
			state:           nil,
			requested:       4 * time.Hour,
			now:             windowStart,
			wantPausedUntil: windowStart.Add(3 * time.Hour),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			newState, err := Begin(test.state, test.requested, test.now)
			if err != nil {
				t.Fatalf("expected the pause to be trimmed and allowed, got: %v", err)
			}
			if !newState.PausedUntil.Equal(test.wantPausedUntil) {
				t.Errorf("paused until: got %v, want %v", newState.PausedUntil, test.wantPausedUntil)
			}
		})
	}
}
