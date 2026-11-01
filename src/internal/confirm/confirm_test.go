package confirm

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestFindFirstDifferenceAcceptsFormattingVariations(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		actual   string
	}{
		{
			name:     "identical input",
			expected: "I want to pause yappblocker.",
			actual:   "I want to pause yappblocker.",
		},
		{
			name:     "different letter case",
			expected: "I accept that now I WILL stay up late.",
			actual:   "i accept that now i will stay up late.",
		},
		{
			name:     "collapsed double spaces",
			expected: "I want to pause yappblocker.",
			actual:   "I  want   to  pause yappblocker.",
		},
		{
			name:     "leading and trailing whitespace",
			expected: "I want to pause yappblocker.",
			actual:   "   I want to pause yappblocker.  ",
		},
		{
			name:     "curly apostrophe substituted for straight",
			expected: "it's not an excuse",
			actual:   "it’s not an excuse",
		},
		{
			name:     "curly double quotes substituted for straight",
			expected: "the word \"pause\" matters",
			actual:   "the word “pause” matters",
		},
		{
			name:     "em dash substituted for hyphen",
			expected: "my long-term goals",
			actual:   "my long—term goals",
		},
		{
			name:     "non-breaking space substituted for space",
			expected: "short-term dopamine",
			actual:   "short-term dopamine",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if diff := FindFirstDifference(test.expected, test.actual); diff != nil {
				t.Errorf("expected a match, but got difference at word %d: expected %q, got %q",
					diff.WordIndex, diff.ExpectedWord, diff.ActualWord)
			}
		})
	}
}

func TestFindFirstDifferenceRejectsChangedWords(t *testing.T) {
	tests := []struct {
		name             string
		expected         string
		actual           string
		wantWordIndex    int
		wantExpectedWord string
		wantActualWord   string
	}{
		{
			name:             "misspelled word",
			expected:         "self-harming behaviours and bad decisions",
			actual:           "self-harming behavious and bad decisions",
			wantWordIndex:    1,
			wantExpectedWord: "behaviours",
			wantActualWord:   "behavious",
		},
		{
			name:             "missing hyphen",
			expected:         "my long-term goals",
			actual:           "my longterm goals",
			wantWordIndex:    1,
			wantExpectedWord: "long-term",
			wantActualWord:   "longterm",
		},
		{
			name:             "missing trailing punctuation",
			expected:         "I want to pause yappblocker.",
			actual:           "I want to pause yappblocker",
			wantWordIndex:    4,
			wantExpectedWord: "yappblocker.",
			wantActualWord:   "yappblocker",
		},
		{
			name:             "input ends early",
			expected:         "Knowing all this, I want to uninstall yappblocker.",
			actual:           "Knowing all this,",
			wantWordIndex:    3,
			wantExpectedWord: "I",
			wantActualWord:   "",
		},
		{
			name:             "input has extra words",
			expected:         "I want to pause yappblocker.",
			actual:           "I want to pause yappblocker. Please.",
			wantWordIndex:    5,
			wantExpectedWord: "",
			wantActualWord:   "please.",
		},
		{
			name:             "empty input",
			expected:         "I want to pause yappblocker.",
			actual:           "",
			wantWordIndex:    0,
			wantExpectedWord: "I",
			wantActualWord:   "",
		},
		{
			name:             "dropped word in the middle",
			expected:         "I am putting myself at risk",
			actual:           "I am putting myself risk",
			wantWordIndex:    4,
			wantExpectedWord: "at",
			wantActualWord:   "risk",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diff := FindFirstDifference(test.expected, test.actual)
			if diff == nil {
				t.Fatal("expected a difference, but the input was accepted")
			}
			if diff.WordIndex != test.wantWordIndex {
				t.Errorf("word index: got %d, want %d", diff.WordIndex, test.wantWordIndex)
			}
			if !strings.EqualFold(diff.ExpectedWord, test.wantExpectedWord) {
				t.Errorf("expected word: got %q, want %q", diff.ExpectedWord, test.wantExpectedWord)
			}
			if !strings.EqualFold(diff.ActualWord, test.wantActualWord) {
				t.Errorf("actual word: got %q, want %q", diff.ActualWord, test.wantActualWord)
			}
		})
	}
}

func TestMinTypingDurationScalesWithPhraseLength(t *testing.T) {
	tests := []struct {
		name   string
		phrase string
		want   time.Duration
	}{
		{
			name:   "empty phrase takes no time",
			phrase: "",
			want:   0,
		},
		{
			name:   "140 characters at 14 chars per second",
			phrase: strings.Repeat("a", 140),
			want:   10 * time.Second,
		},
		{
			name:   "280 characters at 14 chars per second",
			phrase: strings.Repeat("a", 280),
			want:   20 * time.Second,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := MinTypingDuration(test.phrase); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

// The thresholds below are the ones promised to the user when these phrases were
// agreed on. A phrase edit that moves them materially should be a deliberate choice,
// so this test fails rather than letting the promise drift silently.
func TestConfirmationPhrasesHaveTheExpectedTypingFloors(t *testing.T) {
	tests := []struct {
		name    string
		phrase  string
		wantMin time.Duration
		wantMax time.Duration
	}{
		{
			name:    "uninstall phrase takes at least ~40s to type",
			phrase:  UninstallPhrase,
			wantMin: 38 * time.Second,
			wantMax: 43 * time.Second,
		},
		{
			name:    "pause phrase takes at least ~13s to type",
			phrase:  PausePhrase,
			wantMin: 11 * time.Second,
			wantMax: 15 * time.Second,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := MinTypingDuration(test.phrase)
			if got < test.wantMin || got > test.wantMax {
				t.Errorf("got %v, want between %v and %v", got, test.wantMin, test.wantMax)
			}
		})
	}
}

// Both phrases were corrected before shipping: "behavious" to "behaviours", and
// "longterm" to the hyphenated "long-term" used elsewhere in the same sentence.
// Reintroducing either spelling would make the phrase trip the user who types the
// word correctly, so it is worth failing the build over.
func TestConfirmationPhrasesUseTheCorrectedSpellings(t *testing.T) {
	tests := []struct {
		name          string
		phrase        string
		forbiddenWord string
	}{
		{
			name:          "pause phrase spells behaviours correctly",
			phrase:        PausePhrase,
			forbiddenWord: "behavious ",
		},
		{
			name:          "uninstall phrase hyphenates long-term consistently",
			phrase:        UninstallPhrase,
			forbiddenWord: "longterm",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if strings.Contains(test.phrase, test.forbiddenWord) {
				t.Errorf("phrase contains the uncorrected spelling %q", test.forbiddenWord)
			}
		})
	}
}

func TestDescribeDifferenceExplainsWhereTheInputWentWrong(t *testing.T) {
	tests := []struct {
		name       string
		difference *Difference
		wantPhrase string
	}{
		{
			name:       "changed word names both words and a 1-based position",
			difference: &Difference{WordIndex: 1, ExpectedWord: "behaviours", ActualWord: "behavious"},
			wantPhrase: "word 2",
		},
		{
			name:       "input ending early says the phrase is unfinished",
			difference: &Difference{WordIndex: 3, ExpectedWord: "I", ActualWord: ""},
			wantPhrase: "not finished",
		},
		{
			name:       "extra words are called out as extra",
			difference: &Difference{WordIndex: 5, ExpectedWord: "", ActualWord: "please."},
			wantPhrase: "extra words",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := describeDifference(test.difference)
			if !strings.Contains(strings.ToLower(got), test.wantPhrase) {
				t.Errorf("got %q, which does not mention %q", got, test.wantPhrase)
			}
		})
	}
}

func TestPromptRefusesNonInteractiveInput(t *testing.T) {
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatalf("could not create a pipe: %v", err)
	}
	defer readEnd.Close()

	if _, err := writeEnd.WriteString(PausePhrase + "\n"); err != nil {
		t.Fatalf("could not write the phrase to the pipe: %v", err)
	}
	writeEnd.Close()

	output := &strings.Builder{}
	if err := Prompt(readEnd, output, PausePhrase); err == nil {
		t.Error("expected piped input to be refused, but the phrase was accepted")
	}
}

// fixedClock advances by a set step on every reading, standing in for the wall clock
// so that the paste check can be exercised without a test that actually waits.
type fixedClock struct {
	currentTime time.Time
	step        time.Duration
}

func (c *fixedClock) now() time.Time {
	readingTime := c.currentTime
	c.currentTime = c.currentTime.Add(c.step)
	return readingTime
}

func TestPromptUntilTypedAcceptsAPhraseTypedAtHumanSpeed(t *testing.T) {
	clock := &fixedClock{currentTime: time.Now(), step: time.Minute}
	output := &strings.Builder{}

	err := promptUntilTyped(strings.NewReader(PausePhrase+"\n"), output, PausePhrase, clock.now)
	if err != nil {
		t.Fatalf("expected the phrase to be accepted, got error: %v", err)
	}
}

func TestPromptUntilTypedRejectsAPhraseThatArrivedTooFast(t *testing.T) {
	clock := &fixedClock{currentTime: time.Now(), step: time.Second}
	output := &strings.Builder{}

	// Two identical correct attempts: the first is too fast to be typed, and the
	// second is too, so the reader runs dry and the prompt gives up.
	typedInput := PausePhrase + "\n" + PausePhrase + "\n"

	err := promptUntilTyped(strings.NewReader(typedInput), output, PausePhrase, clock.now)
	if err == nil {
		t.Fatal("expected pasted input to be refused, but it was accepted")
	}
	if !strings.Contains(output.String(), "faster than the phrase can be typed") {
		t.Errorf("expected the user to be told why the input was refused, got:\n%s", output.String())
	}
}

func TestPromptUntilTypedReprompsAfterATypoAndThenAccepts(t *testing.T) {
	clock := &fixedClock{currentTime: time.Now(), step: time.Minute}
	output := &strings.Builder{}

	typedInput := "I want to pause yappblocker. I understand nothing.\n" + PausePhrase + "\n"

	err := promptUntilTyped(strings.NewReader(typedInput), output, PausePhrase, clock.now)
	if err != nil {
		t.Fatalf("expected the corrected second attempt to be accepted, got error: %v", err)
	}
	if !strings.Contains(output.String(), "Word 8 should be") {
		t.Errorf("expected feedback naming the first wrong word, got:\n%s", output.String())
	}
}

func TestPromptUntilTypedFailsWhenInputEndsWithoutAMatch(t *testing.T) {
	clock := &fixedClock{currentTime: time.Now(), step: time.Minute}
	output := &strings.Builder{}

	err := promptUntilTyped(strings.NewReader(""), output, PausePhrase, clock.now)
	if err == nil {
		t.Fatal("expected empty input to fail rather than be accepted")
	}
}

// /dev/null is a character device, so a naive "is this a character device" check
// mistakes it for a terminal. Nothing can be typed at /dev/null, so it must be
// refused the same way a pipe is.
func TestPromptRefusesACharacterDeviceThatIsNotATerminal(t *testing.T) {
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("could not open %s: %v", os.DevNull, err)
	}
	defer devNull.Close()

	output := &strings.Builder{}
	err = Prompt(devNull, output, PausePhrase)
	if err == nil {
		t.Fatal("expected a non-terminal character device to be refused")
	}
	if !strings.Contains(err.Error(), "interactive terminal") {
		t.Errorf("expected the refusal to be about needing a terminal, got: %v", err)
	}
}
