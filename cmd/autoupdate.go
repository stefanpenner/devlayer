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
	if os.Getenv("DEVLAYER_NO_AUTOUPDATE") != "" {
		return
	}

	if currentVersion == "dev" {
		return
	}

	stampFile := autoUpdateStampFile()
	if !shouldAutoUpdate(stampFile) {
		return
	}

	// Write the stamp immediately so a failure doesn't cause repeated
	// attempts on every invocation.
	writeStamp(stampFile)

	release, err := getLatestRelease()
	if err != nil {
		return // network errors are silently ignored
	}

	if sameVersion(release.TagName, currentVersion) {
		return
	}

	fmt.Fprint(os.Stderr, formatNotice(currentVersion, release.TagName, release.Body))
}

func autoUpdateStampFile() string {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, _ := os.UserHomeDir()
		if runtime.GOOS == "windows" {
			if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
				dataHome = localAppData
			} else {
				dataHome = filepath.Join(home, "AppData", "Local")
			}
		} else {
			dataHome = filepath.Join(home, ".local", "share")
		}
	}
	return filepath.Join(dataHome, "devlayer", "last-update-check")
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
