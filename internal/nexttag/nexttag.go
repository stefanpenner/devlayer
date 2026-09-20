package nexttag

import (
	"fmt"
	"strconv"
	"strings"
)

// Next returns the next version tag.
// No commits → last unchanged. Any feat/feat! subject → bump minor. Else patch.
// Never auto-bumps major.
func Next(last string, subjects []string) (string, error) {
	major, minor, patch, err := parse(last)
	if err != nil {
		return "", err
	}
	if len(subjects) == 0 {
		return fmt.Sprintf("v%d.%d.%d", major, minor, patch), nil
	}
	if hasFeat(subjects) {
		return fmt.Sprintf("v%d.%d.0", major, minor+1), nil
	}
	return fmt.Sprintf("v%d.%d.%d", major, minor, patch+1), nil
}

func parse(tag string) (major, minor, patch int, err error) {
	s := strings.TrimPrefix(strings.TrimSpace(tag), "v")
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("not a vMAJOR.MINOR.PATCH tag: %q", tag)
	}
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("not a vMAJOR.MINOR.PATCH tag: %q", tag)
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("not a vMAJOR.MINOR.PATCH tag: %q", tag)
	}
	patch, err = strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("not a vMAJOR.MINOR.PATCH tag: %q", tag)
	}
	return major, minor, patch, nil
}

func hasFeat(subjects []string) bool {
	for _, s := range subjects {
		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, "feat:") || strings.HasPrefix(s, "feat(") || strings.HasPrefix(s, "feat!") {
			return true
		}
	}
	return false
}
