package launchd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

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

func getPlistFilePath() (string, error) {
	homeDirPath, err := os.UserHomeDir()
	if err != nil {
		return "", stacktrace.Propagate(err, "could not determine home directory")
	}
	return filepath.Join(homeDirPath, "Library", "LaunchAgents", plistFileName), nil
}

func getLogFilePath() (string, error) {
	homeDirPath, err := os.UserHomeDir()
	if err != nil {
		return "", stacktrace.Propagate(err, "could not determine home directory")
	}
	return filepath.Join(homeDirPath, ".local", "state", "yappblocker", "yappblocker.log"), nil
}

func Install() error {
	binaryFilePath, err := exec.LookPath("yappblocker")
	if err != nil {
		return stacktrace.Propagate(err, "could not find yappblocker in PATH")
	}

	logFilePath, err := getLogFilePath()
	if err != nil {
		return err
	}
	logDirPath := filepath.Dir(logFilePath)
	if err := os.MkdirAll(logDirPath, 0755); err != nil {
		return stacktrace.Propagate(err, "failed to create log directory %q", logDirPath)
	}

	data := plistData{
		Label:          plistLabel,
		BinaryFilePath: binaryFilePath,
		RunInterval:    runInterval,
		LogFilePath:    logFilePath,
	}

	tmpl, err := template.New("plist").Parse(plistTemplate)
	if err != nil {
		return stacktrace.Propagate(err, "failed to parse plist template")
	}

	plistFilePath, err := getPlistFilePath()
	if err != nil {
		return err
	}
	plistDirPath := filepath.Dir(plistFilePath)
	if err := os.MkdirAll(plistDirPath, 0755); err != nil {
		return stacktrace.Propagate(err, "failed to create LaunchAgents directory")
	}

	// Unload existing plist if present (ignore errors)
	if _, statErr := os.Stat(plistFilePath); statErr == nil {
		exec.Command("launchctl", "unload", plistFilePath).Run()
	}

	f, err := os.Create(plistFilePath)
	if err != nil {
		return stacktrace.Propagate(err, "failed to create plist file %q", plistFilePath)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return stacktrace.Propagate(err, "failed to write plist")
	}

	cmd := exec.Command("launchctl", "load", plistFilePath)
	if out, loadErr := cmd.CombinedOutput(); loadErr != nil {
		return stacktrace.Propagate(loadErr, "launchctl load failed: %s", string(out))
	}

	fmt.Fprintf(os.Stderr, "Installed launchd agent: %s\n", plistFilePath)
	fmt.Fprintf(os.Stderr, "yappblocker will run every %d seconds.\n", runInterval)
	fmt.Fprintf(os.Stderr, "Logs: %s\n", logFilePath)
	return nil
}

func Uninstall() error {
	plistFilePath, err := getPlistFilePath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(plistFilePath); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "No launchd agent found — nothing to uninstall.")
		return nil
	}

	cmd := exec.Command("launchctl", "unload", plistFilePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: launchctl unload failed: %s\n", string(out))
	}

	if err := os.Remove(plistFilePath); err != nil {
		return stacktrace.Propagate(err, "failed to remove plist file %q", plistFilePath)
	}

	fmt.Fprintf(os.Stderr, "Uninstalled launchd agent: %s\n", plistFilePath)
	return nil
}
