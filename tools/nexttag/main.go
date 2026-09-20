package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/stefanpenner/devlayer/internal/nexttag"
)

func main() {
	if d := os.Getenv("BUILD_WORKING_DIRECTORY"); d != "" {
		_ = os.Chdir(d)
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	last, err := git("describe", "--tags", "--abbrev=0")
	if err != nil {
		last = "v0.0.0"
	}
	log, err := git("log", last+"..HEAD", "--pretty=%s")
	if err != nil {
		return err
	}
	var subjects []string
	for _, line := range strings.Split(log, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			subjects = append(subjects, line)
		}
	}
	tag, err := nexttag.Next(last, subjects)
	if err != nil {
		return err
	}
	fmt.Println(tag)
	return nil
}

func git(args ...string) (string, error) {
	var out, errb bytes.Buffer
	cmd := exec.Command("git", args...)
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, errb.String())
	}
	return strings.TrimSpace(out.String()), nil
}
