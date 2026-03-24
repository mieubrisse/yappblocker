package config

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Parsing tests
// ---------------------------------------------------------------------------

func TestSingleAppParsesCorrectly(t *testing.T) {
	yaml := `
apps:
  slack:
    match: "Slack"
    killType: "pkillGraceful"
`
	cfg, err := Load(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	app, ok := cfg.Apps["slack"]
	if !ok {
		t.Fatal("expected app 'slack' to exist")
	}
	if app.Name != "slack" {
		t.Errorf("expected Name 'slack', got %q", app.Name)
	}
	if app.Match != "Slack" {
		t.Errorf("expected Match 'Slack', got %q", app.Match)
	}
	if app.KillType != KillTypePkillGraceful {
		t.Errorf("expected KillType %q, got %q", KillTypePkillGraceful, app.KillType)
	}
}

func TestMultipleAppsParse(t *testing.T) {
	yaml := `
apps:
  slack:
    match: "Slack"
    killType: "osascript"
  discord:
    match: "Discord"
    killType: "pkillForce"
`
	cfg, err := Load(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(cfg.Apps))
	}
	if cfg.Apps["slack"].Match != "Slack" {
		t.Errorf("slack match mismatch")
	}
	if cfg.Apps["discord"].Match != "Discord" {
		t.Errorf("discord match mismatch")
	}
}

func TestInvalidKillTypeReturnsError(t *testing.T) {
	yaml := `
apps:
  slack:
    match: "Slack"
    killType: "nuke"
`
	_, err := Load(yaml)
	if err == nil {
		t.Fatal("expected error for invalid killType")
	}
	if !strings.Contains(err.Error(), "nuke") {
		t.Errorf("error should mention invalid killType value, got: %v", err)
	}
}

func TestMissingMatchReturnsError(t *testing.T) {
	yaml := `
apps:
  slack:
    killType: "osascript"
`
	_, err := Load(yaml)
	if err == nil {
		t.Fatal("expected error for missing match")
	}
	if !strings.Contains(err.Error(), "match") {
		t.Errorf("error should mention 'match', got: %v", err)
	}
}

func TestDefaultKillTypeIsOsascript(t *testing.T) {
	yaml := `
apps:
  slack:
    match: "Slack"
`
	cfg, err := Load(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Apps["slack"].KillType != KillTypeOsascript {
		t.Errorf("expected default KillType %q, got %q", KillTypeOsascript, cfg.Apps["slack"].KillType)
	}
}

// ---------------------------------------------------------------------------
// App set resolution tests
// ---------------------------------------------------------------------------

func TestResolveSetWithAppsOnly(t *testing.T) {
	yaml := `
apps:
  slack:
    match: "Slack"
  discord:
    match: "Discord"
appSets:
  social:
    apps:
      - slack
      - discord
`
	cfg, err := Load(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	apps, err := cfg.ResolveAppSet("social")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(apps))
	}

	names := map[string]bool{}
	for _, a := range apps {
		names[a.Name] = true
	}
	if !names["slack"] || !names["discord"] {
		t.Errorf("expected slack and discord, got %v", names)
	}
}

func TestResolveSetWithNestedSets(t *testing.T) {
	yaml := `
apps:
  slack:
    match: "Slack"
  discord:
    match: "Discord"
appSets:
  chat:
    apps:
      - slack
  social:
    apps:
      - discord
    appSets:
      - chat
`
	cfg, err := Load(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	apps, err := cfg.ResolveAppSet("social")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(apps))
	}
}

func TestResolveSetWithBothAppsAndSets(t *testing.T) {
	yaml := `
apps:
  slack:
    match: "Slack"
  discord:
    match: "Discord"
  zoom:
    match: "zoom.us"
appSets:
  chat:
    apps:
      - slack
  distractions:
    apps:
      - discord
      - zoom
    appSets:
      - chat
`
	cfg, err := Load(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	apps, err := cfg.ResolveAppSet("distractions")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 3 {
		t.Fatalf("expected 3 apps, got %d", len(apps))
	}
}

func TestCircularReferenceDetected(t *testing.T) {
	yaml := `
apps:
  slack:
    match: "Slack"
appSets:
  a:
    apps:
      - slack
    appSets:
      - b
  b:
    appSets:
      - a
`
	cfg, err := Load(yaml)
	if err != nil {
		t.Fatalf("unexpected error during load: %v", err)
	}
	_, err = cfg.ResolveAppSet("a")
	if err == nil {
		t.Fatal("expected error for circular reference")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Errorf("error should mention 'circular', got: %v", err)
	}
}

func TestUnknownAppReferenceReturnsError(t *testing.T) {
	yaml := `
apps:
  slack:
    match: "Slack"
appSets:
  social:
    apps:
      - slack
      - nonexistent
`
	_, err := Load(yaml)
	if err == nil {
		t.Fatal("expected error for unknown app reference")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention 'nonexistent', got: %v", err)
	}
}

func TestUnknownSetReferenceReturnsError(t *testing.T) {
	yaml := `
apps:
  slack:
    match: "Slack"
appSets:
  social:
    apps:
      - slack
    appSets:
      - nonexistent
`
	_, err := Load(yaml)
	if err == nil {
		t.Fatal("expected error for unknown set reference")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention 'nonexistent', got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Full config load tests
// ---------------------------------------------------------------------------

func TestFullConfigLoadsCorrectly(t *testing.T) {
	yaml := `
apps:
  slack:
    match: "Slack"
    killType: "osascript"
  discord:
    match: "Discord"
    killType: "pkillGraceful"
appSets:
  social:
    apps:
      - slack
      - discord
schedules:
  workday:
    appSet: "social"
    windows:
      - days: ["monday", "tuesday", "wednesday", "thursday", "friday"]
        start: "09:00"
        end: "17:00"
`
	cfg, err := Load(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Apps) != 2 {
		t.Errorf("expected 2 apps, got %d", len(cfg.Apps))
	}
	if len(cfg.AppSets) != 1 {
		t.Errorf("expected 1 appSet, got %d", len(cfg.AppSets))
	}
	if len(cfg.Schedules) != 1 {
		t.Errorf("expected 1 schedule, got %d", len(cfg.Schedules))
	}

	sched := cfg.Schedules["workday"]
	if sched.AppSet != "social" {
		t.Errorf("expected appSet 'social', got %q", sched.AppSet)
	}
	if len(sched.Windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(sched.Windows))
	}
	w := sched.Windows[0]
	if len(w.Days) != 5 {
		t.Errorf("expected 5 days, got %d", len(w.Days))
	}
	if w.Start != "09:00" {
		t.Errorf("expected start '09:00', got %q", w.Start)
	}
	if w.End != "17:00" {
		t.Errorf("expected end '17:00', got %q", w.End)
	}
}

func TestScheduleReferencingUnknownAppSetReturnsError(t *testing.T) {
	yaml := `
apps:
  slack:
    match: "Slack"
schedules:
  workday:
    appSet: "nonexistent"
    windows:
      - days: ["monday"]
        start: "09:00"
        end: "17:00"
`
	_, err := Load(yaml)
	if err == nil {
		t.Fatal("expected error for schedule referencing unknown appSet")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention 'nonexistent', got: %v", err)
	}
}
