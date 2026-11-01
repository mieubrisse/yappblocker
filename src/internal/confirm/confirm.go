// Package confirm gates a command behind a phrase the user has to type out by hand.
//
// The point is friction, not security: anyone with a shell can unload the launchd
// agent directly. What this stops is the absent-minded, impulsive version of the
// action — the one the tool exists to resist.
package confirm

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mieubrisse/stacktrace"
	"golang.org/x/term"
)

// maxPlausibleTypingCharsPerSecond is the fastest sustained typing speed a human is
// credibly capable of. 14 characters per second is about 168 words per minute —
// comfortably beyond any real typist, so a genuine attempt is never rejected for
// being quick, while a paste is caught every time.
const maxPlausibleTypingCharsPerSecond = 14.0

// substitutedCharacters maps characters that keyboards and editors silently
// substitute onto the plain ASCII the phrases are written in, so a smart quote or a
// non-breaking space cannot make a phrase impossible to type correctly.
var substitutedCharacters = map[rune]rune{
	'‘': '\'', // left single quote
	'’': '\'', // right single quote / curly apostrophe
	'“': '"',  // left double quote
	'”': '"',  // right double quote
	'–': '-',  // en dash
	'—': '-',  // em dash
	' ': ' ',  // non-breaking space
}

// Difference describes the first place where typed input diverges from a phrase.
type Difference struct {
	// WordIndex is the 0-based position of the diverging word.
	WordIndex int
	// ExpectedWord is the word that belongs at that position, empty when the input
	// carried on past the end of the phrase.
	ExpectedWord string
	// ActualWord is the word that was typed there, empty when the input ended early.
	ActualWord string
}

// FindFirstDifference reports where typed input first diverges from the expected
// phrase, or nil when the two match. Letter case, runs of whitespace, and
// substituted quotes and dashes are all ignored; every word and every piece of
// punctuation still has to be there.
func FindFirstDifference(expected, actual string) *Difference {
	expectedWords := splitIntoComparableWords(expected)
	actualWords := splitIntoComparableWords(actual)

	for i := 0; i < len(expectedWords) || i < len(actualWords); i++ {
		expectedWord := ""
		if i < len(expectedWords) {
			expectedWord = expectedWords[i]
		}

		actualWord := ""
		if i < len(actualWords) {
			actualWord = actualWords[i]
		}

		if !strings.EqualFold(expectedWord, actualWord) {
			return &Difference{
				WordIndex:    i,
				ExpectedWord: expectedWord,
				ActualWord:   actualWord,
			}
		}
	}

	return nil
}

// MinTypingDuration is how long the given phrase must take to arrive before it is
// believable that someone typed it. Input that lands faster was pasted.
func MinTypingDuration(phrase string) time.Duration {
	seconds := float64(len(phrase)) / maxPlausibleTypingCharsPerSecond
	return time.Duration(seconds * float64(time.Second))
}

// Prompt shows the phrase and makes the user type it back, returning nil only once
// the input matches and took long enough to have been typed rather than pasted. It
// re-prompts on a mistake so a typo near the end does not cost a full retype.
func Prompt(in *os.File, out io.Writer, phrase string) error {
	if !isInteractiveTerminal(in) {
		// Plain errors rather than stacktrace errors in this function and the loop
		// below: these messages are instructions for a person, and a source location
		// appended to them only makes them harder to read.
		return errors.New(
			"this command has to be run from an interactive terminal, because it requires typing a confirmation phrase by hand",
		)
	}

	return promptUntilTyped(in, out, phrase, time.Now)
}

// promptUntilTyped is the prompting loop, with the clock injected so the paste check
// can be tested without a test that has to wait out a real typing floor.
func promptUntilTyped(in io.Reader, out io.Writer, phrase string, nowFunc func() time.Time) error {
	minTypingDuration := MinTypingDuration(phrase)

	fmt.Fprintf(out, "\nType this phrase to continue, or press Ctrl-C to abort:\n\n%s\n\n", phrase)
	fmt.Fprintf(out, "Capitalization and spacing do not matter; every word does. Typing it takes about %v,\nand input arriving faster than that is rejected as a paste.\n\n", minTypingDuration.Round(time.Second))

	scanner := bufio.NewScanner(in)
	for {
		fmt.Fprint(out, "> ")

		startTime := nowFunc()
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return stacktrace.Propagate(err, "failed to read the confirmation phrase from the terminal")
			}
			return errors.New("aborted before the confirmation phrase was typed")
		}
		elapsed := nowFunc().Sub(startTime)

		if difference := FindFirstDifference(phrase, scanner.Text()); difference != nil {
			fmt.Fprintf(out, "\n%s\nType the whole phrase again.\n\n", describeDifference(difference))
			continue
		}

		if elapsed < minTypingDuration {
			fmt.Fprintf(out, "\nThat arrived in %v, which is faster than the phrase can be typed. Type it out instead of pasting it.\n\n", elapsed.Round(time.Second))
			continue
		}

		return nil
	}
}

// splitIntoComparableWords normalizes away the formatting differences that should not
// count as mistakes, and splits what remains into words. Case is preserved so that
// feedback can quote the phrase as it was written.
func splitIntoComparableWords(s string) []string {
	substituted := strings.Map(func(r rune) rune {
		if replacement, found := substitutedCharacters[r]; found {
			return replacement
		}
		return r
	}, s)

	return strings.Fields(substituted)
}

// describeDifference turns a divergence into a sentence telling the user where they
// went wrong, using a 1-based word position to match how a person counts.
func describeDifference(difference *Difference) string {
	wordPosition := difference.WordIndex + 1

	if difference.ActualWord == "" {
		return fmt.Sprintf("The phrase is not finished — word %d should be %q.", wordPosition, difference.ExpectedWord)
	}

	if difference.ExpectedWord == "" {
		return fmt.Sprintf("There are extra words on the end, starting with %q at word %d.", difference.ActualWord, wordPosition)
	}

	return fmt.Sprintf("Word %d should be %q, but you typed %q.", wordPosition, difference.ExpectedWord, difference.ActualWord)
}

// isInteractiveTerminal reports whether the given input is a terminal rather than a
// pipe, a file, or another character device, so that feeding the phrase in cannot
// stand in for typing it. This asks the operating system directly instead of
// checking for a character device, which would wrongly accept /dev/null.
func isInteractiveTerminal(in *os.File) bool {
	return term.IsTerminal(int(in.Fd()))
}
