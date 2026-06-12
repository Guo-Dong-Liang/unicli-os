// Package runner provides a Docker-based sandbox runner for CPL tools.
// It applies the security constraints defined in the CPL spec (§6):
// no network, read-only rootfs, dropped capabilities, non-root user.
package runner

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Guo-Dong-Liang/unicli-os/pkg/cpl"
)

// SandboxRunner runs CPL tools inside a Docker container with full
// sandbox isolation.
type SandboxRunner struct {
	Manifest     *cpl.CPLManifest
	DockerBin    string // Path to docker binary (auto-detected: "docker")
	DockerHost   string // DOCKER_HOST override (auto-detected if empty)
	PipeMode     bool   // Suppress banner, connect stdin/stdout directly
	AllowNetwork bool   // Override manifest's network setting
	ExtraArgs    []string // Extra docker run args (e.g. -v mounts)
}

// RunResult captures the outcome of a sandboxed run.
type RunResult struct {
	ExitCode int
	Duration time.Duration
}

// Run executes the tool in a Docker sandbox and returns the result.
// It connects stdin/stdout/stderr from the host process.
func (r *SandboxRunner) Run() (*RunResult, error) {
	if r.Manifest == nil {
		return nil, errors.New("runner: manifest is required")
	}
	imageRef := r.Manifest.Image.Ref
	if imageRef == "" {
		return nil, errors.New("runner: image ref is required")
	}

	dockerBin := r.DockerBin
	if dockerBin == "" {
		dockerBin = "docker"
	}

	args := r.buildDockerArgs()

	cmd := exec.Command(dockerBin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	dockerHost := r.resolveDockerHost()
	if dockerHost != "" {
		cmd.Env = append(os.Environ(), "DOCKER_HOST="+dockerHost)
	}

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return &RunResult{ExitCode: -1, Duration: duration}, fmt.Errorf("runner: failed to run container: %w", err)
		}
	}

	return &RunResult{ExitCode: exitCode, Duration: duration}, nil
}

// RunWithOutput executes the tool and captures stdout/stderr (for pipe mode).
func (r *SandboxRunner) RunWithOutput(stdin io.Reader) (stdout, stderr string, exitCode int, err error) {
	if r.Manifest == nil {
		return "", "", -1, errors.New("runner: manifest is required")
	}
	imageRef := r.Manifest.Image.Ref
	if imageRef == "" {
		return "", "", -1, errors.New("runner: image ref is required")
	}

	dockerBin := r.DockerBin
	if dockerBin == "" {
		dockerBin = "docker"
	}

	args := r.buildDockerArgs()

	cmd := exec.Command(dockerBin, args...)

	var outBuf, errBuf strings.Builder
	cmd.Stdin = stdin
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	dockerHost := r.resolveDockerHost()
	if dockerHost != "" {
		cmd.Env = append(os.Environ(), "DOCKER_HOST="+dockerHost)
	}

	runErr := cmd.Run()
	exitCode = 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return "", "", -1, fmt.Errorf("runner: failed to run container: %w", runErr)
		}
	}

	return outBuf.String(), errBuf.String(), exitCode, nil
}

// buildDockerArgs constructs docker run arguments from the manifest.
func (r *SandboxRunner) buildDockerArgs() []string {
	args := []string{"run", "--rm", "-i"}

	// Apply CPL sandbox security defaults (§6)
	if !r.AllowNetwork && !r.Manifest.Resources.Network {
		args = append(args, "--network", "none")
	}
	args = append(args, "--read-only")
	args = append(args, "--cap-drop=ALL")
	args = append(args, "--user", "nobody:nogroup")

	// Resource limits from manifest
	if r.Manifest.Resources.Memory > 0 {
		args = append(args, "--memory", fmt.Sprintf("%dm", r.Manifest.Resources.Memory))
	}
	if r.Manifest.Resources.CPU > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%f", r.Manifest.Resources.CPU))
	}
	if r.Manifest.Resources.Timeout > 0 {
		args = append(args, "--stop-timeout", fmt.Sprintf("%d", r.Manifest.Resources.Timeout))
	}

	// Extra args (e.g. volume mounts for development)
	args = append(args, r.ExtraArgs...)

	// Image and entrypoint
	args = append(args, r.Manifest.Image.Ref)
	if r.Manifest.Image.Entrypoint != "" {
		// Allow manifest entrypoint to override Docker CMD
		// We pass it as a command override to docker run
		args = append(args, "/bin/sh", "-c", r.Manifest.Image.Entrypoint)
	}

	return args
}

// resolveDockerHost detects the Docker socket path for the current platform.
func (r *SandboxRunner) resolveDockerHost() string {
	if r.DockerHost != "" {
		return r.DockerHost
	}

	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost != "" {
		return dockerHost
	}

	home, _ := os.UserHomeDir()
	var candidates []string

	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			filepath.Join(home, "Library", "Containers", "com.docker.docker", "Data", "docker.raw.sock"),
			"/var/run/docker.sock",
		}
	case "windows":
		candidates = []string{"//./pipe/docker_engine"}
	default:
		candidates = []string{
			"/var/run/docker.sock",
			filepath.Join(home, ".docker", "run", "docker.sock"),
		}
	}

	for _, sock := range candidates {
		if _, err := os.Stat(sock); err == nil {
			if strings.HasPrefix(sock, "//") {
				return "" // Named pipe — let Docker CLI resolve it
			}
			return "unix://" + sock
		}
	}

	return ""
}
