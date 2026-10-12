package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/stefanpenner/devlayer/internal/download"
	"github.com/stefanpenner/devlayer/internal/hostapp"
)

// Host runs one pinned program that stays outside the sysroot.
func Host(args []string) error {
	if len(args) == 0 || args[0] != hostapp.Name {
		return fmt.Errorf("host: the app is %s", hostapp.Name)
	}

	data, err := hostDataHome()
	if err != nil {
		return err
	}
	plan, err := hostapp.Plan(data, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	if plan.Fetch {
		fmt.Printf("download %s %s\n", hostapp.Name, hostapp.Version)
		if err := fetchHost(plan); err != nil {
			return err
		}
		plan, err = hostapp.Plan(data, runtime.GOOS, runtime.GOARCH)
		if err != nil {
			return err
		}
		if plan.Fetch || plan.Exec == "" {
			return fmt.Errorf("host: %s is not ready", hostapp.Name)
		}
	}
	return execApp(plan.Exec, args[1:])
}

func hostDataHome() (string, error) {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share"), nil
}

func fetchHost(plan hostapp.Decision) error {
	if err := os.RemoveAll(plan.Cache); err != nil {
		return err
	}
	if err := os.MkdirAll(plan.Cache, 0755); err != nil {
		return err
	}
	if err := download.TarGzFull(plan.Pin.URL, plan.Cache, 1, plan.Pin.Sum); err != nil {
		os.RemoveAll(plan.Cache)
		return err
	}
	if err := hostapp.WriteStamp(plan.Cache, plan.Pin.Sum); err != nil {
		os.RemoveAll(plan.Cache)
		return err
	}
	return nil
}
