package schedule

import (
	"sort"
	"time"

	"github.com/mieubrisse/yappblocker/internal/config"
)

// ActiveBlocking describes blocking that is in force at a given moment.
type ActiveBlocking struct {
	// ScheduleNames holds the name of every schedule with an open window, sorted so
	// that what the user sees is stable from one run to the next.
	ScheduleNames []string
	// EndsAt is when the last of those windows closes — the moment blocking lifts.
	EndsAt time.Time
}

// FindActiveBlocking reports the blocking in force at the given time, or nil when
// nothing is being blocked. Windows whose times cannot be parsed count as closed,
// which is how IsWindowActive already treats them.
func FindActiveBlocking(cfg *config.Config, now time.Time) *ActiveBlocking {
	var scheduleNames []string
	var latestEnd time.Time

	for scheduleName, sched := range cfg.Schedules {
		isScheduleActive := false

		// Every open window is considered, not just the first one found: a schedule
		// may list overlapping windows, and stopping early would report blocking as
		// lifting at the earliest of their ends rather than the latest.
		for _, window := range sched.Windows {
			if !IsWindowActive(window, now) {
				continue
			}

			windowEnd, err := windowEndTime(window, now)
			if err != nil {
				continue
			}

			isScheduleActive = true
			if windowEnd.After(latestEnd) {
				latestEnd = windowEnd
			}
		}

		if isScheduleActive {
			scheduleNames = append(scheduleNames, scheduleName)
		}
	}

	if len(scheduleNames) == 0 {
		return nil
	}

	sort.Strings(scheduleNames)

	return &ActiveBlocking{
		ScheduleNames: scheduleNames,
		EndsAt:        latestEnd,
	}
}

// windowEndTime returns the moment an open window closes. It assumes the window is
// open at the given time, which is the only way FindActiveBlocking calls it.
func windowEndTime(window config.WindowDef, now time.Time) (time.Time, error) {
	startHour, startMinute, err := parseTime(window.Start)
	if err != nil {
		return time.Time{}, err
	}
	endHour, endMinute, err := parseTime(window.End)
	if err != nil {
		return time.Time{}, err
	}

	endTimeToday := time.Date(now.Year(), now.Month(), now.Day(), endHour, endMinute, 0, 0, now.Location())

	startMinutes := startHour*60 + startMinute
	endMinutes := endHour*60 + endMinute
	isOvernightWindow := startMinutes >= endMinutes
	hasPassedTodaysStart := now.Hour()*60+now.Minute() >= startMinutes

	// An overnight window entered before midnight closes on the following morning;
	// one entered after midnight closes on this morning.
	if isOvernightWindow && hasPassedTodaysStart {
		return endTimeToday.AddDate(0, 0, 1), nil
	}

	return endTimeToday, nil
}
