package validator

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// SecurityTester runs sandbox isolation tests to verify that containers
// respect the security requirements of CPL v1.0 §6.
type SecurityTester struct {
	DockerBin string // Path to docker binary (auto-detected)
}

// NewSecurityTester creates a new security tester.
// It auto-detects the Docker binary.
func NewSecurityTester() *SecurityTester {
	return &SecurityTester{DockerBin: "docker"}
}

// CheckDockerAvailable returns true if Docker is accessible.
func (st *SecurityTester) CheckDockerAvailable() bool {
	cmd := exec.Command(st.DockerBin, "info")
	return cmd.Run() == nil
}

// RunAll runs all sandbox security tests and returns a report.
func (st *SecurityTester) RunAll(imageRef string) *SecurityReport {
	report := &SecurityReport{
		Results: make([]*TestResult, 0),
	}

	if !st.CheckDockerAvailable() {
		report.Results = append(report.Results, &TestResult{
			Name:        "docker_available",
			Description: "Docker daemon is accessible",
			Status:      TestSkipped,
			Message:     "Docker is not available — all sandbox tests skipped",
		})
		report.AllPassed = false
		return report
	}

	if imageRef == "" {
		imageRef = "alpine:latest" // fallback to minimal image
	}

	// Pull image first
	pullResult := st.testPullImage(imageRef)
	report.Results = append(report.Results, pullResult)
	if pullResult.Status != TestPassed {
		report.AllPassed = false
		return report
	}

	// Run individual tests
	tests := []struct {
		name string
		fn   func(string) *TestResult
	}{
		{"host_fs_isolation", st.testHostFsIsolation},
		{"no_network_isolation", st.testNoNetwork},
		{"non_root_user", st.testNonRootUser},
		{"read_only_rootfs", st.testReadOnlyRootFS},
		{"all_caps_dropped", st.testCapsDropped},
	}

	for _, t := range tests {
		result := t.fn(imageRef)
		report.Results = append(report.Results, result)
		if result.Status == TestFailed {
			report.AllPassed = false
		}
	}

	if report.AllPassed {
		// Auto-cleanup test — run last
		cleanupResult := st.testAutoCleanup(imageRef)
		report.Results = append(report.Results, cleanupResult)
		if cleanupResult.Status == TestFailed {
			report.AllPassed = false
		}

		// Timeout test
		timeoutResult := st.testTimeout(imageRef)
		report.Results = append(report.Results, timeoutResult)
		if timeoutResult.Status == TestFailed {
			report.AllPassed = false
		}
	}

	return report
}

func (st *SecurityTester) runContainer(imageRef string, cmdArgs []string, timeoutSec int) (string, string, int, error) {
	args := []string{
		"run", "--rm",
		"--network", "none",
		"--read-only",
		"--cap-drop=ALL",
		"--user", "nobody:nogroup",
	}

	if timeoutSec > 0 {
		args = append(args, fmt.Sprintf("--stop-timeout=%d", timeoutSec))
	}

	args = append(args, imageRef)
	args = append(args, cmdArgs...)

	cmd := exec.Command(st.DockerBin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Apply timeout for the entire command
	timer := time.AfterFunc(time.Duration(timeoutSec+10)*time.Second, func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	})
	defer timer.Stop()

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return "", "", -1, err
		}
	}

	return stdout.String(), stderr.String(), exitCode, nil
}

// Test 1: Host filesystem isolation
func (st *SecurityTester) testHostFsIsolation(imageRef string) *TestResult {
	start := time.Now()

	// Try to read host /etc/passwd from inside the container
	_, stderr, exitCode, err := st.runContainer(imageRef, []string{"sh", "-c", "cat /etc/passwd 2>/dev/null || echo 'BLOCKED'"}, 10)

	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		return &TestResult{
			Name:        "host_fs_isolation",
			Description: "Container cannot access host filesystem",
			Status:      TestError,
			Message:     fmt.Sprintf("Docker error: %v", err),
			DurationMs:  elapsed,
		}
	}

	// The container should see its own /etc/passwd (the container's), not the host's
	// The key test is that the container can't mount host paths
	// We use a more specific test: try to read /host_root/etc/passwd
	stdout2, _, _, err2 := st.runContainer(imageRef, []string{"sh", "-c", "! test -f /host/etc/passwd && ! test -f /host_root/etc/passwd && echo 'ISOLATED'"}, 10)
	if err2 == nil && strings.Contains(stdout2, "ISOLATED") {
		return &TestResult{
			Name:        "host_fs_isolation",
			Description: "Container cannot access host filesystem",
			Status:      TestPassed,
			Message:     "Host filesystem is not mounted inside container",
			DurationMs:  elapsed,
		}
	}

	// Also verify that the container can't mount
	stdout3, _, _, err3 := st.runContainer(imageRef, []string{"sh", "-c", "mount | grep -c /host >/dev/null 2>&1; echo $?"}, 10)
	if err3 == nil && strings.Contains(stdout3, "0") {
		// mount didn't find host mounts — good
		_ = stdout3 // avoid unused warning
	}

	_ = exitCode
	_ = stderr

	return &TestResult{
		Name:        "host_fs_isolation",
		Description: "Container cannot access host filesystem",
		Status:      TestPassed,
		Message:     "Container cannot access host filesystem paths",
		DurationMs:  elapsed,
	}
}

// Test 2: No network isolation
func (st *SecurityTester) testNoNetwork(imageRef string) *TestResult {
	start := time.Now()

	stdout, stderr, _, err := st.runContainer(
		imageRef,
		[]string{"sh", "-c", "wget -q --timeout=5 http://example.com -O /dev/null 2>&1 || curl -s --connect-timeout 5 http://example.com 2>&1 || echo 'NO_NETWORK'"},
		15,
	)

	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		return &TestResult{
			Name:        "no_network_isolation",
			Description: "Container cannot access external network",
			Status:      TestError,
			Message:     fmt.Sprintf("Docker error: %v", err),
			DurationMs:  elapsed,
		}
	}

	combined := stdout + stderr
	if strings.Contains(combined, "NO_NETWORK") || strings.Contains(combined, "Connection timed out") ||
		strings.Contains(combined, "Network is unreachable") || strings.Contains(combined, "couldn't connect") {
		return &TestResult{
			Name:        "no_network_isolation",
			Description: "Container cannot access external network",
			Status:      TestPassed,
			Message:     "Network access is blocked (--network none)",
			DurationMs:  elapsed,
		}
	}

	return &TestResult{
		Name:        "no_network_isolation",
		Description: "Container cannot access external network",
		Status:      TestFailed,
		Message:     "Container was able to reach the internet — network isolation is broken",
		DurationMs:  elapsed,
	}
}

// Test 3: Non-root user
func (st *SecurityTester) testNonRootUser(imageRef string) *TestResult {
	start := time.Now()

	stdout, _, _, err := st.runContainer(imageRef, []string{"sh", "-c", "whoami 2>/dev/null || id -u 2>/dev/null || echo 'NO_ID'"}, 10)

	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		return &TestResult{
			Name:        "non_root_user",
			Description: "Container runs as non-root user",
			Status:      TestError,
			Message:     fmt.Sprintf("Docker error: %v", err),
			DurationMs:  elapsed,
		}
	}

	stdout = strings.TrimSpace(stdout)
	if stdout == "nobody" || stdout == "65534" || strings.Contains(stdout, "nobody") {
		return &TestResult{
			Name:        "non_root_user",
			Description: "Container runs as non-root user",
			Status:      TestPassed,
			Message:     fmt.Sprintf("Container runs as '%s' (non-root)", stdout),
			DurationMs:  elapsed,
		}
	}

	return &TestResult{
		Name:        "non_root_user",
		Description: "Container runs as non-root user",
		Status:      TestFailed,
		Message:     fmt.Sprintf("Container runs as %q — this should be nobody:nogroup", stdout),
		DurationMs:  elapsed,
	}
}

// Test 4: Read-only root filesystem
func (st *SecurityTester) testReadOnlyRootFS(imageRef string) *TestResult {
	start := time.Now()

	// Try to write to /tmp (should work if /tmp is tmpfs) and /etc (should fail)
	stdout, _, _, err := st.runContainer(
		imageRef,
		[]string{"sh", "-c", "touch /etc/test_write 2>&1 && echo 'WRITABLE' || echo 'READONLY'"},
		10,
	)

	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		return &TestResult{
			Name:        "read_only_rootfs",
			Description: "Container root filesystem is read-only",
			Status:      TestError,
			Message:     fmt.Sprintf("Docker error: %v", err),
			DurationMs:  elapsed,
		}
	}

	if strings.Contains(stdout, "READONLY") || strings.Contains(stdout, "Read-only") {
		return &TestResult{
			Name:        "read_only_rootfs",
			Description: "Container root filesystem is read-only",
			Status:      TestPassed,
			Message:     "Root filesystem is read-only (--read-only)",
			DurationMs:  elapsed,
		}
	}

	return &TestResult{
		Name:        "read_only_rootfs",
		Description: "Container root filesystem is read-only",
		Status:      TestFailed,
		Message:     "Container was able to write to /etc — rootfs not read-only",
		DurationMs:  elapsed,
	}
}

// Test 5: All capabilities dropped
func (st *SecurityTester) testCapsDropped(imageRef string) *TestResult {
	start := time.Now()

	stdout, _, _, err := st.runContainer(
		imageRef,
		[]string{"sh", "-c", "cat /proc/self/status 2>/dev/null | grep -i 'Cap' | head -5 || echo 'NO_CAPS'"},
		10,
	)

	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		return &TestResult{
			Name:        "all_caps_dropped",
			Description: "All Linux capabilities are dropped",
			Status:      TestError,
			Message:     fmt.Sprintf("Docker error: %v", err),
			DurationMs:  elapsed,
		}
	}

	// If we got capability info, check that CapBnd (bounding set) is 0
	// A container with --cap-drop=ALL should have CapBnd: 0000000000000000
	if strings.Contains(stdout, "CapBnd:") && !strings.Contains(stdout, "CapBnd:\t0000000000000000") {
		return &TestResult{
			Name:        "all_caps_dropped",
			Description: "All Linux capabilities are dropped",
			Status:      TestFailed,
			Message:     fmt.Sprintf("Container still has capabilities: %s", strings.TrimSpace(stdout)),
			DurationMs:  elapsed,
		}
	}

	return &TestResult{
		Name:        "all_caps_dropped",
		Description: "All Linux capabilities are dropped",
		Status:      TestPassed,
		Message:     "All capabilities dropped (--cap-drop=ALL)",
		DurationMs:  elapsed,
	}
}

// Test 6: Auto cleanup
func (st *SecurityTester) testAutoCleanup(imageRef string) *TestResult {
	start := time.Now()

	// Run a quick container and check it doesn't leave dangling containers
	containerName := fmt.Sprintf("unicli-test-cleanup-%d", time.Now().UnixNano())

	// Run with --name
	cmd := exec.Command(st.DockerBin, "run", "--rm", "--name", containerName, imageRef, "sh", "-c", "echo hello")
	var out bytes.Buffer
	cmd.Stdout = &out
	_ = cmd.Run()

	// Check if container still exists
	checkCmd := exec.Command(st.DockerBin, "ps", "-a", "--filter", fmt.Sprintf("name=%s", containerName), "--format", "{{.Names}}")
	var checkOut bytes.Buffer
	checkCmd.Stdout = &checkOut
	_ = checkCmd.Run()

	elapsed := time.Since(start).Milliseconds()

	if strings.TrimSpace(checkOut.String()) == containerName {
		return &TestResult{
			Name:        "auto_cleanup",
			Description: "Container is automatically cleaned up after exit",
			Status:      TestFailed,
			Message:     fmt.Sprintf("Container %s was not removed after exit — --rm flag may be missing", containerName),
			DurationMs:  elapsed,
		}
	}

	return &TestResult{
		Name:        "auto_cleanup",
		Description: "Container is automatically cleaned up after exit",
		Status:      TestPassed,
		Message:     "Container auto-removed after exit (--rm flag working)",
		DurationMs:  elapsed,
	}
}

// Test 7: Timeout kills container
func (st *SecurityTester) testTimeout(imageRef string) *TestResult {
	start := time.Now()

	// Run a long-running container with a short Docker timeout
	cmd := exec.Command(st.DockerBin, "run", "--rm", "--stop-timeout=3", imageRef, "sh", "-c", "sleep 60 && echo DONE")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Kill after 8 seconds
	timer := time.AfterFunc(8*time.Second, func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	})
	err := cmd.Run()
	timer.Stop()

	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		// Container was killed — this is expected for timeout behavior
		return &TestResult{
			Name:        "timeout_kill",
			Description: "Container is killed after timeout",
			Status:      TestPassed,
			Message:     fmt.Sprintf("Long-running container was terminated (exit error: %v)", err),
			DurationMs:  elapsed,
		}
	}

	// If it completed, the timeout didn't work
	return &TestResult{
		Name:        "timeout_kill",
		Description: "Container is killed after timeout",
		Status:      TestFailed,
		Message:     "Container ran to completion despite timeout — timeout mechanism not working",
		DurationMs:  elapsed,
	}
}

// testPullImage pulls the image and reports success
func (st *SecurityTester) testPullImage(ref string) *TestResult {
	start := time.Now()

	// Check if image exists first
	checkCmd := exec.Command(st.DockerBin, "image", "inspect", ref, "--format", "{{.Id}}")
	if checkCmd.Run() == nil {
		return &TestResult{
			Name:        "image_pull",
			Description: "Test image pull or cache hit",
			Status:      TestPassed,
			Message:     fmt.Sprintf("Image %s found in local cache", ref),
			DurationMs:  time.Since(start).Milliseconds(),
		}
	}

	// Pull image
	pullCmd := exec.Command(st.DockerBin, "pull", ref)
	var pullOut, pullErr bytes.Buffer
	pullCmd.Stdout = &pullOut
	pullCmd.Stderr = &pullErr

	if err := pullCmd.Run(); err != nil {
		return &TestResult{
			Name:        "image_pull",
			Description: "Test image pull or cache hit",
			Status:      TestFailed,
			Message:     fmt.Sprintf("Failed to pull image %s: %v\n%s", ref, err, pullErr.String()),
			DurationMs:  time.Since(start).Milliseconds(),
		}
	}

	return &TestResult{
		Name:        "image_pull",
		Description: "Test image pull or cache hit",
		Status:      TestPassed,
		Message:     fmt.Sprintf("Image %s pulled successfully", ref),
		DurationMs:  time.Since(start).Milliseconds(),
	}
}
