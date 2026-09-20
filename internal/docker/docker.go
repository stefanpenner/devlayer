package docker

import (
	"fmt"
	"os"
	"os/exec"
)

// Build runs docker build with the given platform, tag, and context directory.
func Build(platform, tag, contextDir string, buildArgs map[string]string) error {
	args := []string{"build", "--platform", platform, "-t", tag}
	for k, v := range buildArgs {
		args = append(args, "--build-arg", k+"="+v)
	}
	args = append(args, contextDir)
	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RunToFile runs a container and redirects stdout to a file.
func RunToFile(image, outputPath string) error {
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	cmd := exec.Command("docker", "run", "--rm", image)
	cmd.Stdout = f
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RemoveImage removes a docker image.
func RemoveImage(image string) error {
	cmd := exec.Command("docker", "rmi", image)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ImageExists checks if a docker image exists locally.
func ImageExists(image string) bool {
	cmd := exec.Command("docker", "image", "inspect", image)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// PrintSize prints the size of a file.
func PrintSize(path string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	mb := float64(info.Size()) / 1024 / 1024
	fmt.Printf("  %s (%.0f MB)\n", path, mb)
}
