package config

import (
	"github.com/mieubrisse/stacktrace"
	"gopkg.in/yaml.v3"
)

// KillType determines how an application is terminated.
type KillType string

const (
	KillTypeOsascript     KillType = "osascript"
	KillTypePkillGraceful KillType = "pkillGraceful"
	KillTypePkillForce    KillType = "pkillForce"
)

var validKillTypes = map[KillType]bool{
	KillTypeOsascript:     true,
	KillTypePkillGraceful: true,
	KillTypePkillForce:    true,
}

type App struct {
	Name     string
	Match    string   `yaml:"match"`
	KillType KillType `yaml:"killType"`
}

type AppSetDef struct {
	Apps    []string `yaml:"apps"`
	AppSets []string `yaml:"appSets"`
}

type WindowDef struct {
	Days  []string `yaml:"days"`
	Start string   `yaml:"start"`
	End   string   `yaml:"end"`
}

type ScheduleDef struct {
	AppSet  string      `yaml:"appSet"`
	Windows []WindowDef `yaml:"windows"`
}

type Config struct {
	Apps      map[string]*App         `yaml:"apps"`
	AppSets   map[string]*AppSetDef   `yaml:"appSets"`
	Schedules map[string]*ScheduleDef `yaml:"schedules"`
}

// Load parses YAML into a Config, sets App.Name from map keys, and validates
// all references and constraints.
func Load(yamlStr string) (*Config, error) {
	cfg := &Config{}
	if err := yaml.Unmarshal([]byte(yamlStr), cfg); err != nil {
		return nil, stacktrace.Propagate(err, "failed to parse YAML")
	}

	if err := validateApps(cfg); err != nil {
		return nil, err
	}

	if err := validateAppSets(cfg); err != nil {
		return nil, err
	}

	if err := validateSchedules(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func validateApps(cfg *Config) error {
	for name, app := range cfg.Apps {
		app.Name = name

		if app.Match == "" {
			return stacktrace.NewError("app %q has empty match", name)
		}

		if app.KillType == "" {
			app.KillType = KillTypeOsascript
		} else if !validKillTypes[app.KillType] {
			return stacktrace.NewError("app %q has invalid killType %q", name, app.KillType)
		}
	}
	return nil
}

func validateAppSets(cfg *Config) error {
	for setName, setDef := range cfg.AppSets {
		for _, appName := range setDef.Apps {
			if _, ok := cfg.Apps[appName]; !ok {
				return stacktrace.NewError("appSet %q references unknown app %q", setName, appName)
			}
		}
		for _, refSetName := range setDef.AppSets {
			if _, ok := cfg.AppSets[refSetName]; !ok {
				return stacktrace.NewError("appSet %q references unknown appSet %q", setName, refSetName)
			}
		}
	}
	return nil
}

func validateSchedules(cfg *Config) error {
	for schedName, sched := range cfg.Schedules {
		if sched.AppSet != "" {
			if _, ok := cfg.AppSets[sched.AppSet]; !ok {
				return stacktrace.NewError("schedule %q references unknown appSet %q", schedName, sched.AppSet)
			}
		}
	}
	return nil
}

// ResolveAppSet recursively resolves an app set to its constituent apps,
// detecting circular references. Apps are deduplicated by name.
func (c *Config) ResolveAppSet(setName string) ([]*App, error) {
	visited := map[string]bool{}
	seen := map[string]bool{}
	var result []*App

	if err := c.resolveAppSetRecursive(setName, visited, seen, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Config) resolveAppSetRecursive(setName string, visited map[string]bool, seen map[string]bool, result *[]*App) error {
	if visited[setName] {
		return stacktrace.NewError("circular reference detected: appSet %q has already been visited", setName)
	}

	setDef, ok := c.AppSets[setName]
	if !ok {
		return stacktrace.NewError("unknown appSet %q", setName)
	}

	visited[setName] = true

	for _, appName := range setDef.Apps {
		if seen[appName] {
			continue
		}
		app, ok := c.Apps[appName]
		if !ok {
			return stacktrace.NewError("appSet %q references unknown app %q", setName, appName)
		}
		seen[appName] = true
		*result = append(*result, app)
	}

	for _, nestedSetName := range setDef.AppSets {
		if err := c.resolveAppSetRecursive(nestedSetName, visited, seen, result); err != nil {
			return stacktrace.Propagate(err, "resolving nested appSet %q from %q", nestedSetName, setName)
		}
	}

	delete(visited, setName)

	return nil
}
