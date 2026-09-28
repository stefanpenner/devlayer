package sysroot

import (
	"fmt"
	"slices"
	"strings"
)

// Package is one library in the sysroot. Depends names packages that must
// be installed before this one is configured.
type Package struct {
	Name    string
	Depends []string
}

// Order returns package names so every dependency appears first.
// Packages that are both ready are emitted in name order, so the result
// does not depend on map iteration.
func Order(pkgs []Package) ([]string, error) {
	byName := make(map[string]Package, len(pkgs))
	indegree := make(map[string]int, len(pkgs))
	users := make(map[string][]string, len(pkgs))
	for _, p := range pkgs {
		if _, ok := byName[p.Name]; ok {
			return nil, fmt.Errorf("duplicate package %s", p.Name)
		}
		byName[p.Name] = p
		indegree[p.Name] = 0
	}
	for _, p := range pkgs {
		for _, dep := range p.Depends {
			if _, ok := byName[dep]; !ok {
				return nil, fmt.Errorf("%s depends on unknown package %s", p.Name, dep)
			}
			indegree[p.Name]++
			users[dep] = append(users[dep], p.Name)
		}
	}

	ready := make([]string, 0, len(pkgs))
	for name, n := range indegree {
		if n == 0 {
			ready = append(ready, name)
		}
	}
	slices.Sort(ready)

	out := make([]string, 0, len(pkgs))
	for len(ready) > 0 {
		name := ready[0]
		ready = ready[1:]
		out = append(out, name)
		slices.Sort(users[name])
		for _, user := range users[name] {
			indegree[user]--
			if indegree[user] == 0 {
				ready = append(ready, user)
				slices.Sort(ready)
			}
		}
	}
	if len(out) != len(pkgs) {
		left := make([]string, 0)
		for name, n := range indegree {
			if n > 0 {
				left = append(left, name)
			}
		}
		slices.Sort(left)
		return nil, fmt.Errorf("cycle among %s", strings.Join(left, ", "))
	}
	return out, nil
}
