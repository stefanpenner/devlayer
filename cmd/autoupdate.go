package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const autoUpdateInterval = 24 * time.Hour

// AutoUpdate checks for a newer GitHub release at most once per day.
// It prints a notice (and release notes if present). It never installs;
// run `devlayer upgrade` for that.
func AutoUpdate(currentVersion string) {
	if os.Getenv("DEVLAYER_NO_AUTOUPDATE") != "" || currentVersion == "dev" {
		return
	}

	stampFile := autoUpdateStampFile()
	if !shouldAutoUpdate(stampFile) {
		return
	}

	// Stamp before the fetch so a network failure does not retry every command.
	writeStamp(stampFile)

	release, err := getLatestRelease()
	if err != nil {
		return
	}
	if sameVersion(release.TagName, currentVersion) {
		return
	}

	fmt.Fprint(os.Stderr, formatNotice(currentVersion, release.TagName, release.Body))
}

func autoUpdateStampFile() string {
	return filepath.Join(dataHome(), "devlayer", "last-update-check")
}

func dataHome() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return dir
	}
	if runtime.GOOS == "windows" {
		if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
			return dir
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "AppData", "Local")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share")
}

func shouldAutoUpdate(stampFile string) bool {
	data, err := os.ReadFile(stampFile)
	if err != nil {
		return true
	}

	ts, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return true
	}

	return time.Since(time.Unix(ts, 0)) >= autoUpdateInterval
}

func writeStamp(stampFile string) {
	os.MkdirAll(filepath.Dir(stampFile), 0755)
	os.WriteFile(stampFile, []byte(strconv.FormatInt(time.Now().Unix(), 10)), 0644)
}
