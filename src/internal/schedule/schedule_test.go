package schedule

import (
	"testing"
	"time"

	"github.com/mieubrisse/yappblocker/internal/config"
)

func TestIsWindowActive(t *testing.T) {
	tests := []struct {
		name   string
		window config.WindowDef
		now    time.Time
		want   bool
	}{
		{
			name:   "same-day window active within range",
			window: config.WindowDef{Days: []string{"mon", "tue", "wed", "thu"}, Start: "20:45", End: "23:00"},
			now:    time.Date(2026, 3, 25, 21, 0, 0, 0, time.Local), // Wednesday
			want:   true,
		},
		{
			name:   "same-day window inactive before start",
			window: config.WindowDef{Days: []string{"mon", "tue", "wed", "thu"}, Start: "20:45", End: "23:00"},
			now:    time.Date(2026, 3, 25, 20, 30, 0, 0, time.Local), // Wednesday
			want:   false,
		},
		{
			name:   "same-day window inactive wrong day",
			window: config.WindowDef{Days: []string{"mon", "tue", "wed", "thu"}, Start: "20:45", End: "23:00"},
			now:    time.Date(2026, 3, 28, 21, 0, 0, 0, time.Local), // Saturday
			want:   false,
		},
		{
			name:   "overnight window active before midnight",
			window: config.WindowDef{Days: []string{"mon", "tue", "wed", "thu"}, Start: "20:45", End: "06:00"},
			now:    time.Date(2026, 3, 25, 23, 0, 0, 0, time.Local), // Wednesday night
			want:   true,
		},
		{
			name:   "overnight window active after midnight",
			window: config.WindowDef{Days: []string{"wed"}, Start: "20:45", End: "06:00"},
			now:    time.Date(2026, 3, 26, 2, 0, 0, 0, time.Local), // Thursday 2am, window started Wed
			want:   true,
		},
		{
			name:   "overnight window inactive after end",
			window: config.WindowDef{Days: []string{"wed"}, Start: "20:45", End: "06:00"},
			now:    time.Date(2026, 3, 26, 7, 0, 0, 0, time.Local), // Thursday 7am
			want:   false,
		},
		{
			name:   "overnight window inactive wrong start day",
			window: config.WindowDef{Days: []string{"mon"}, Start: "20:45", End: "06:00"},
			now:    time.Date(2026, 3, 26, 2, 0, 0, 0, time.Local), // Thursday 2am
			want:   false,
		},
		{
			name:   "sunday keyword works",
			window: config.WindowDef{Days: []string{"sun"}, Start: "10:00", End: "22:00"},
			now:    time.Date(2026, 3, 29, 15, 0, 0, 0, time.Local), // Sunday
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsWindowActive(tt.window, tt.now)
			if got != tt.want {
				t.Errorf("IsWindowActive() = %v, want %v", got, tt.want)
			}
		})
	}
}
