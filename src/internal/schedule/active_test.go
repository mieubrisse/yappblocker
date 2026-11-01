package schedule

import (
	"reflect"
	"testing"
	"time"

	"github.com/mieubrisse/yappblocker/internal/config"
)

// 2026-03-25 is a Wednesday, matching the existing schedule tests in this package.
func TestFindActiveBlocking(t *testing.T) {
	tests := []struct {
		name              string
		schedules         map[string]*config.ScheduleDef
		now               time.Time
		wantNil           bool
		wantScheduleNames []string
		wantEndsAt        time.Time
	}{
		{
			name:      "no schedules at all",
			schedules: map[string]*config.ScheduleDef{},
			now:       time.Date(2026, 3, 25, 21, 0, 0, 0, time.Local),
			wantNil:   true,
		},
		{
			name: "window closed right now",
			schedules: map[string]*config.ScheduleDef{
				"soft_shutdown": {
					AppSet:  "games",
					Windows: []config.WindowDef{{Days: []string{"wed"}, Start: "20:00", End: "06:00"}},
				},
			},
			now:     time.Date(2026, 3, 25, 14, 0, 0, 0, time.Local),
			wantNil: true,
		},
		{
			name: "same-day window ends later the same day",
			schedules: map[string]*config.ScheduleDef{
				"work_hours": {
					AppSet:  "games",
					Windows: []config.WindowDef{{Days: []string{"wed"}, Start: "09:00", End: "17:00"}},
				},
			},
			now:               time.Date(2026, 3, 25, 10, 30, 0, 0, time.Local),
			wantScheduleNames: []string{"work_hours"},
			wantEndsAt:        time.Date(2026, 3, 25, 17, 0, 0, 0, time.Local),
		},
		{
			name: "overnight window entered before midnight ends tomorrow morning",
			schedules: map[string]*config.ScheduleDef{
				"soft_shutdown": {
					AppSet:  "games",
					Windows: []config.WindowDef{{Days: []string{"wed"}, Start: "20:00", End: "06:00"}},
				},
			},
			now:               time.Date(2026, 3, 25, 22, 30, 0, 0, time.Local),
			wantScheduleNames: []string{"soft_shutdown"},
			wantEndsAt:        time.Date(2026, 3, 26, 6, 0, 0, 0, time.Local),
		},
		{
			name: "overnight window entered after midnight ends this morning",
			schedules: map[string]*config.ScheduleDef{
				"soft_shutdown": {
					AppSet:  "games",
					Windows: []config.WindowDef{{Days: []string{"wed"}, Start: "20:00", End: "06:00"}},
				},
			},
			now:               time.Date(2026, 3, 26, 2, 0, 0, 0, time.Local),
			wantScheduleNames: []string{"soft_shutdown"},
			wantEndsAt:        time.Date(2026, 3, 26, 6, 0, 0, 0, time.Local),
		},
		{
			name: "several open windows report every name and the latest end",
			schedules: map[string]*config.ScheduleDef{
				"soft_shutdown": {
					AppSet:  "games",
					Windows: []config.WindowDef{{Days: []string{"wed"}, Start: "20:00", End: "06:00"}},
				},
				"journalling_only": {
					AppSet:  "games",
					Windows: []config.WindowDef{{Days: []string{"wed"}, Start: "20:45", End: "07:00"}},
				},
			},
			now:               time.Date(2026, 3, 25, 23, 0, 0, 0, time.Local),
			wantScheduleNames: []string{"journalling_only", "soft_shutdown"},
			wantEndsAt:        time.Date(2026, 3, 26, 7, 0, 0, 0, time.Local),
		},
		{
			name: "one schedule with two open windows reports the later end, not the first found",
			schedules: map[string]*config.ScheduleDef{
				"soft_shutdown": {
					AppSet: "games",
					Windows: []config.WindowDef{
						{Days: []string{"wed"}, Start: "09:00", End: "17:00"},
						{Days: []string{"wed"}, Start: "12:00", End: "20:00"},
					},
				},
			},
			now:               time.Date(2026, 3, 25, 14, 0, 0, 0, time.Local),
			wantScheduleNames: []string{"soft_shutdown"},
			wantEndsAt:        time.Date(2026, 3, 25, 20, 0, 0, 0, time.Local),
		},
		{
			name: "a schedule with two open windows is named only once",
			schedules: map[string]*config.ScheduleDef{
				"soft_shutdown": {
					AppSet: "games",
					Windows: []config.WindowDef{
						{Days: []string{"wed"}, Start: "12:00", End: "20:00"},
						{Days: []string{"wed"}, Start: "09:00", End: "17:00"},
					},
				},
			},
			now:               time.Date(2026, 3, 25, 14, 0, 0, 0, time.Local),
			wantScheduleNames: []string{"soft_shutdown"},
			wantEndsAt:        time.Date(2026, 3, 25, 20, 0, 0, 0, time.Local),
		},
		{
			name: "a schedule with an unparseable window is ignored rather than reported",
			schedules: map[string]*config.ScheduleDef{
				"broken": {
					AppSet:  "games",
					Windows: []config.WindowDef{{Days: []string{"wed"}, Start: "nonsense", End: "06:00"}},
				},
			},
			now:     time.Date(2026, 3, 25, 22, 0, 0, 0, time.Local),
			wantNil: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := &config.Config{Schedules: test.schedules}

			active := FindActiveBlocking(cfg, test.now)

			if test.wantNil {
				if active != nil {
					t.Fatalf("expected no active blocking, got %+v", active)
				}
				return
			}

			if active == nil {
				t.Fatal("expected active blocking, got nil")
			}
			if !reflect.DeepEqual(active.ScheduleNames, test.wantScheduleNames) {
				t.Errorf("schedule names: got %v, want %v", active.ScheduleNames, test.wantScheduleNames)
			}
			if !active.EndsAt.Equal(test.wantEndsAt) {
				t.Errorf("end time: got %v, want %v", active.EndsAt, test.wantEndsAt)
			}
		})
	}
}
