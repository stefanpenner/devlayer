// Package linuxbuild compiles a Linux tool in the Alpine builder.
// pick tool → fetch source → configure → make → strip → tar stdout
package linuxbuild

import (
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// Exec runs a command in dir (empty dir = process cwd).
type Exec func(dir, name string, args ...string) error

// Output runs a command and returns stdout.
type Output func(dir, name string, args ...string) (string, error)

// Tools is the recipe list.
func Tools() []string {
	return []string{"git", "zsh", "htop", "btop", "nvim", "make"}
}

// Run executes the recipe. Logs to stderr. Writes tar.gz bytes to stdout.
// env is process env (GIT_VERSION, HTOP_VERSION, ...). nproc is runtime.NumCPU().
func Run(tool string, env map[string]string, stdout io.Writer, stderr io.Writer) error {
	return dispatch(tool, osExec(stdout, stderr), osOutput(stderr), env, stdout)
}

func dispatch(tool string, x Exec, out Output, env map[string]string, stdout io.Writer) error {
	switch tool {
	case "git":
		return Git(x, out, env, stdout)
	case "zsh":
		return Zsh(x, out, env, stdout)
	case "htop":
		return Htop(x, out, env, stdout)
	case "btop":
		return Btop(x, out, env, stdout)
	case "nvim":
		return Nvim(x, out, env, stdout)
	case "make":
		return Make(x, out, env, stdout)
	default:
		return fmt.Errorf("unknown tool %q", tool)
	}
}

func requireEnv(env map[string]string, key string) (string, error) {
	v := ""
	if env != nil {
		v = env[key]
	}
	if v == "" {
		return "", fmt.Errorf("missing %s", key)
	}
	return v, nil
}

func jobs() string {
	return "-j" + strconv.Itoa(runtime.NumCPU())
}

func fetchTarball(x Exec, url, dest string) error {
	if err := x("", "curl", "-fsSL", "-o", dest, url); err != nil {
		return err
	}
	return x("", "tar", "xzf", dest)
}

func tarPrefix(x Exec, prefix string) error {
	return x("", "tar", "czf", "-", "-C", prefix, ".")
}

func osExec(stdout, stderr io.Writer) Exec {
	return func(dir, name string, args ...string) error {
		cmd := exec.Command(name, args...)
		if dir != "" {
			cmd.Dir = dir
		}
		cmd.Stderr = stderr
		if tarWritesStdout(name, args) {
			cmd.Stdout = stdout
		} else {
			cmd.Stdout = stderr
		}
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return nil
	}
}

func osOutput(stderr io.Writer) Output {
	return func(dir, name string, args ...string) (string, error) {
		cmd := exec.Command(name, args...)
		if dir != "" {
			cmd.Dir = dir
		}
		cmd.Stderr = stderr
		b, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return string(b), nil
	}
}

func tarWritesStdout(name string, args []string) bool {
	if name != "tar" {
		return false
	}
	for i, a := range args {
		if (a == "czf" || a == "-czf") && i+1 < len(args) && args[i+1] == "-" {
			return true
		}
	}
	return false
}
