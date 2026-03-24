package schedule

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mieubrisse/yappblocker/internal/config"
)

// dayNames maps the adjusted weekday index to the short day name used in config.
var dayNames = [7]string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

// IsWindowActive reports whether the given time falls within the schedule window.
func IsWindowActive(window config.WindowDef, now time.Time) bool {
	startHour, startMinute, err := parseTime(window.Start)
	if err != nil {
		return false
	}
	endHour, endMinute, err := parseTime(window.End)
	if err != nil {
		return false
	}

	todayDay := dayName(now)
	nowMinutes := now.Hour()*60 + now.Minute()
	startMinutes := startHour*60 + startMinute
	endMinutes := endHour*60 + endMinute

	if startMinutes < endMinutes {
		// Same-day window: active if today is a listed day and time is within [start, end).
		return containsDay(window.Days, todayDay) &&
			nowMinutes >= startMinutes &&
			nowMinutes < endMinutes
	}

	// Overnight window (start >= end): spans midnight.
	// Active if (today is a listed day and time >= start)
	// OR (yesterday is a listed day and time < end).
	if containsDay(window.Days, todayDay) && nowMinutes >= startMinutes {
		return true
	}

	yesterdayDay := prevDayName(now)
	if containsDay(window.Days, yesterdayDay) && nowMinutes < endMinutes {
		return true
	}

	return false
}

// dayName returns the short lowercase day name for the given time.
func dayName(t time.Time) string {
	// Go: Sunday=0 .. Saturday=6
	// We want: mon=0, tue=1 ... sun=6
	// Formula: (weekday + 6) % 7
	idx := (int(t.Weekday()) + 6) % 7
	return dayNames[idx]
}

// prevDayName returns the short lowercase day name for the day before the given time.
func prevDayName(t time.Time) string {
	return dayName(t.AddDate(0, 0, -1))
}

// parseTime parses an "HH:MM" string into hour and minute components.
// Returns an error on malformed input instead of panicking.
func parseTime(s string) (hour, minute int, err error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time format %q: expected HH:MM", s)
	}

	hour, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid hour in %q: %v", s, err)
	}

	minute, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid minute in %q: %v", s, err)
	}

	return hour, minute, nil
}

// containsDay checks whether the given day name appears in the list.
func containsDay(days []string, target string) bool {
	for _, d := range days {
		if d == target {
			return true
		}
	}
	return false
}
