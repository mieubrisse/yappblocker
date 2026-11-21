package launchd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/template"

	"github.com/adrg/xdg"
	"github.com/mieubrisse/stacktrace"
)

const (
	plistLabel    = "com.yappblocker"
	plistFileName = plistLabel + ".plist"
	runInterval   = 120
)

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{.Label}}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.BinaryFilePath}}</string>
        <string>run</string>
    </array>
    <key>StartInterval</key>
    <integer>{{.RunInterval}}</integer>
    <key>StandardOutPath</key>
    <string>{{.LogFilePath}}</string>
    <key>StandardErrorPath</key>
    <string>{{.LogFilePath}}</string>
    <key>RunAtLoad</key>
    <true/>
</dict>
</plist>
`

type plistData struct {
	Label          string
	BinaryFilePath string
	RunInterval    int
	LogFilePath    string
}

// CommandRunner abstracts command execution for testability, so that no test has
// to run the real launchctl against the user's live agent.
type CommandRunner interface {
	Run(args []string) (stdout string, err error)
}

// Agent is the launchd agent that runs yappblocker on a timer: where its plist and
// log live, and how launchctl is reached.
type Agent struct {
	plistFilePath string
	logFilePath   string
	runner        CommandRunner
}

func NewAgent(plistFilePath string, logFilePath string, runner CommandRunner) *Agent {
	return &Agent{
		plistFilePath: plistFilePath,
		logFilePath:   logFilePath,
		runner:        runner,
	}
}

// NewUserAgent returns the agent at its real location in the user's home.
func NewUserAgent(runner CommandRunner) (*Agent, error) {
	homeDirPath, err := os.UserHomeDir()
	if err != nil {
		return nil, stacktrace.Propagate(err, "could not determine home directory")
	}
	plistFilePath := filepath.Join(homeDirPath, "Library", "LaunchAgents", plistFileName)
	logFilePath := filepath.Join(xdg.StateHome, "yappblocker", "yappblocker.log")
	return NewAgent(plistFilePath, logFilePath, runner), nil
}

// Install writes the plist pointing at the given binary and loads it, replacing any
// agent already loaded from the same path.
func (a *Agent) Install(binaryFilePath string, out io.Writer) error {
	logDirPath := filepath.Dir(a.logFilePath)
	if err := os.MkdirAll(logDirPath, 0755); err != nil {
		return stacktrace.Propagate(err, "failed to create log directory %q", logDirPath)
	}

	plistContents, err := renderPlist(binaryFilePath, a.logFilePath)
	if err != nil {
		return err
	}

	plistDirPath := filepath.Dir(a.plistFilePath)
	if err := os.MkdirAll(plistDirPath, 0755); err != nil {
		return stacktrace.Propagate(err, "failed to create LaunchAgents directory %q", plistDirPath)
	}

	// Unload existing plist if present (ignore errors)
	if _, statErr := os.Stat(a.plistFilePath); statErr == nil {
		_, _ = a.runner.Run([]string{"launchctl", "unload", a.plistFilePath})
	}

	if err := os.WriteFile(a.plistFilePath, plistContents, 0644); err != nil {
		return stacktrace.Propagate(err, "failed to write plist file %q", a.plistFilePath)
	}

	if loadOutput, err := a.runner.Run([]string{"launchctl", "load", a.plistFilePath}); err != nil {
		return stacktrace.Propagate(err, "launchctl load failed: %s", loadOutput)
	}

	fmt.Fprintf(out, "Installed launchd agent: %s\n", a.plistFilePath)
	fmt.Fprintf(out, "yappblocker will run every %d seconds.\n", runInterval)
	fmt.Fprintf(out, "Logs: %s\n", a.logFilePath)
	return nil
}

// Uninstall unloads the agent and removes its plist. A missing plist is not an
// error, since there is then nothing left to remove.
func (a *Agent) Uninstall(out io.Writer) error {
	if _, err := os.Stat(a.plistFilePath); os.IsNotExist(err) {
		fmt.Fprintln(out, "No launchd agent found — nothing to uninstall.")
		return nil
	}

	if unloadOutput, err := a.runner.Run([]string{"launchctl", "unload", a.plistFilePath}); err != nil {
		fmt.Fprintf(out, "warning: launchctl unload failed: %s\n", unloadOutput)
	}

	if err := os.Remove(a.plistFilePath); err != nil {
		return stacktrace.Propagate(err, "failed to remove plist file %q", a.plistFilePath)
	}

	fmt.Fprintf(out, "Uninstalled launchd agent: %s\n", a.plistFilePath)
	return nil
}

func renderPlist(binaryFilePath string, logFilePath string) ([]byte, error) {
	tmpl, err := template.New("plist").Parse(plistTemplate)
	if err != nil {
		return nil, stacktrace.Propagate(err, "failed to parse plist template")
	}

	data := plistData{
		Label:          plistLabel,
		BinaryFilePath: binaryFilePath,
		RunInterval:    runInterval,
		LogFilePath:    logFilePath,
	}

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data); err != nil {
		return nil, stacktrace.Propagate(err, "failed to render plist for binary %q and log %q", binaryFilePath, logFilePath)
	}
	return rendered.Bytes(), nil
}
