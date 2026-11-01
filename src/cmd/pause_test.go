package cmd

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "whole hours drop the minutes",
			duration: 3 * time.Hour,
			want:     "3h",
		},
		{
			name:     "hours and minutes together",
			duration: 2*time.Hour + 15*time.Minute,
			want:     "2h15m",
		},
		{
			name:     "under an hour is minutes alone",
			duration: 45 * time.Minute,
			want:     "45m",
		},
		{
			name:     "seconds round to the nearest minute",
			duration: 44*time.Minute + 40*time.Second,
			want:     "45m",
		},
		{
			name:     "a single minute",
			duration: time.Minute,
			want:     "1m",
		},
		{
			name:     "anything under a minute is spelled out",
			duration: 20 * time.Second,
			want:     "less than a minute",
		},
		{
			name:     "zero is spelled out too",
			duration: 0,
			want:     "less than a minute",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := formatDuration(test.duration); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
