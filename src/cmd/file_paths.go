package cmd

import (
	"path/filepath"

	"github.com/adrg/xdg"
)

// filePaths are the files the commands read and write. The cobra handlers build the
// user's real ones; tests build them under a temporary directory, so no test can
// reach the real config or state.
type filePaths struct {
	configFilePath           string
	pauseStateFilePath       string
	ignoredKillStateFilePath string
}

func newFilePaths(configDirPath string, stateDirPath string) *filePaths {
	return &filePaths{
		configFilePath:           filepath.Join(configDirPath, "config.yaml"),
		pauseStateFilePath:       filepath.Join(stateDirPath, "pause.json"),
		ignoredKillStateFilePath: filepath.Join(stateDirPath, "ignored-kills.json"),
	}
}

func getUserFilePaths() *filePaths {
	return newFilePaths(
		filepath.Join(xdg.ConfigHome, "yappblocker"),
		filepath.Join(xdg.StateHome, "yappblocker"),
	)
}
