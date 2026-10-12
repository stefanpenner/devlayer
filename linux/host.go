package linux

// HostCanRun reports whether this process can execute the bundle.
// A Linux CPU runs only its own bundle.
func HostCanRun(goos, goarch, bundleArch string) bool {
	if goos != "linux" {
		return false
	}
	switch bundleArch {
	case "aarch64":
		return goarch == "arm64"
	case "x86_64":
		return goarch == "amd64"
	default:
		return false
	}
}
