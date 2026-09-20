package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"

	"github.com/stefanpenner/devlayer/internal/binaries"
	"github.com/stefanpenner/devlayer/internal/platform"
	"github.com/stefanpenner/devlayer/internal/versions"
)

func main() {
	if d := os.Getenv("BUILD_WORKING_DIRECTORY"); d != "" {
		_ = os.Chdir(d)
	}

	out := flag.String("out", "", "output directory")
	osName := flag.String("os", runtime.GOOS, "target OS")
	arch := flag.String("arch", defaultArch(), "target arch")
	versPath := flag.String("versions", "versions.env", "versions.env path")
	skip := flag.String("skip", "", "tool to skip (e.g. nvim)")
	probe := flag.Bool("probe", false, "HEAD download URLs; do not fetch")
	flag.Parse()

	if !*probe && *out == "" {
		fmt.Fprintln(os.Stderr, "download: --out is required (or pass --probe)")
		os.Exit(2)
	}

	data, err := os.ReadFile(*versPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "download: %v\n", err)
		os.Exit(1)
	}
	vers := versions.Parse(string(data))

	var skipMap map[string]bool
	if *skip != "" {
		skipMap = map[string]bool{*skip: true}
	}

	if *probe {
		if err := runProbe(*osName, *arch, vers, skipMap); err != nil {
			fmt.Fprintf(os.Stderr, "download: %v\n", err)
			os.Exit(1)
		}
		return
	}

	p, err := platform.New(*osName, *arch)
	if err != nil {
		fmt.Fprintf(os.Stderr, "download: %v\n", err)
		os.Exit(1)
	}

	if err := binaries.Fetch(*out, p, vers, skipMap); err != nil {
		fmt.Fprintf(os.Stderr, "download: %v\n", err)
		os.Exit(1)
	}
}

func runProbe(osName, arch string, vers *versions.Versions, skip map[string]bool) error {
	osSet, archSet := false, false
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "os":
			osSet = true
		case "arch":
			archSet = true
		}
	})
	if osSet || archSet {
		p, err := platform.New(osName, arch)
		if err != nil {
			return err
		}
		urls, err := binaries.URLs(p, vers, skip)
		if err != nil {
			return err
		}
		fmt.Printf("==> Probing %s/%s (%d URLs)\n", p.OS, p.Arch, len(urls))
		return binaries.Probe(urls, httpStatus)
	}

	fmt.Println("==> Probing default platforms")
	return binaries.ProbeTargets(vers, httpStatus, binaries.DefaultProbeTargets())
}

func httpStatus(url string) (int, error) {
	code, err := doStatus(http.MethodHead, url)
	if err != nil {
		return 0, err
	}
	if code == http.StatusOK {
		return code, nil
	}
	if code == http.StatusMethodNotAllowed || code == http.StatusForbidden {
		return doStatus(http.MethodGet, url)
	}
	return code, nil
}

func doStatus(method, url string) (int, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "devlayer-probe")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	return resp.StatusCode, nil
}

func defaultArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	default:
		return runtime.GOARCH
	}
}
