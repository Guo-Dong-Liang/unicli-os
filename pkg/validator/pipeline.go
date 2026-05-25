package validator

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

// PipelineTester runs end-to-end pipeline tests to verify that:
// 1. StructuredMessage frames pass correctly between stages
// 2. Data integrity is preserved across the pipe
// 3. EOS signal terminates properly
type PipelineTester struct {
	DockerBin string
}

// NewPipelineTester creates a new pipeline tester.
func NewPipelineTester() *PipelineTester {
	return &PipelineTester{DockerBin: "docker"}
}

// CheckDockerAvailable checks if Docker is accessible.
func (pt *PipelineTester) CheckDockerAvailable() bool {
	cmd := exec.Command(pt.DockerBin, "info")
	return cmd.Run() == nil
}

// RunBasicPipeTest runs a simple two-stage pipe test using hello.say.
// Stage 1: hello.say --name "World"
// Stage 2: hello.say --greeting "Hi"  (receives Stage 1 output as input)
// Returns the test result.
func (pt *PipelineTester) RunBasicPipeTest(imageRef string) *TestResult {
	start := time.Now()

	if imageRef == "" {
		imageRef = "ghcr.io/unixcli/hello.say:latest"
	}

	if !pt.CheckDockerAvailable() {
		return &TestResult{
			Name:        "basic_pipe",
			Description: "Two-stage pipe: hello.say can be chained",
			Status:      TestSkipped,
			Message:     "Docker is not available",
		}
	}

	// Create a named pipe (FIFO) for staging
	pipeDir, err := os.MkdirTemp("", "unicli-pipe-test-*")
	if err != nil {
		return &TestResult{
			Name:        "basic_pipe",
			Description: "Two-stage pipe: hello.say can be chained",
			Status:      TestError,
			Message:     fmt.Sprintf("Failed to create temp dir: %v", err),
			DurationMs:  time.Since(start).Milliseconds(),
		}
	}
	defer os.RemoveAll(pipeDir)

	// Actual pipe test: run two containers connected via Docker networking
	// Container 1: echo "Hello, World" (simulates output)
	// Container 2: receives via pipe
	//
	// For Phase 1, we test by running hello.say in a container and capturing output
	// Since we don't have the actual hello.say image built yet, we test the
	// compatibility matrix statically and provide a framework for integration tests.

	result := &TestResult{
		Name:        "basic_pipe",
		Description: "Two-stage pipe: data passes through StructuredMessage frames",
		Status:      TestSkipped,
		Message:     "Integration pipe test requires Docker images to be built first (make docker-build-examples)",
		DurationMs:  time.Since(start).Milliseconds(),
	}

	_ = pipeDir // Future: use for FIFO-based pipe testing

	return result
}

// RunCompatibilityMatrixTest validates that all entries in the pipe compatibility
// matrix (cpl-spec-v1.md §5.3) return correct results.
func (pt *PipelineTester) RunCompatibilityMatrixTest() *TestResult {
	start := time.Now()
	checker := NewPipeCompatibilityChecker()

	// Test all known combinations
	tests := []struct {
		from     string
		to       string
		expected bool // expected compatible
	}{
		// Trivial passes
		{"TEXT", "STRING", true},
		{"TEXT", "FILE", true},
		{"FILE", "FILE", true},
		{"STREAM", "STREAM", true},
		{"STREAM", "STRING", true},
		{"STRUCT", "STRUCT", true},
		{"STRUCT", "STRING", true},

		// Expected failures
		{"FILE", "STRING", false},
		{"FILE", "STREAM", false},
		{"FILE", "INT", false},
		{"FILE", "BOOLEAN", false},
		{"TEXT", "INT", false},
		{"TEXT", "BOOLEAN", false},
		{"STREAM", "BOOLEAN", false},
		{"STRUCT", "INT", false},
	}

	failed := 0
	for _, tc := range tests {
		cr := checker.Check(OutputType(tc.from), InputType(tc.to))
		if cr.Compatible != tc.expected {
			failed++
		}
	}

	elapsed := time.Since(start).Milliseconds()

	if failed > 0 {
		return &TestResult{
			Name:        "pipe_compatibility_matrix",
			Description: "Pipe type compatibility matrix is correct",
			Status:      TestFailed,
			Message:     fmt.Sprintf("%d of %d compatibility checks returned unexpected results", failed, len(tests)),
			DurationMs:  elapsed,
		}
	}

	return &TestResult{
		Name:        "pipe_compatibility_matrix",
		Description: "Pipe type compatibility matrix is correct",
		Status:      TestPassed,
		Message:     fmt.Sprintf("All %d compatibility checks passed", len(tests)),
		DurationMs:  elapsed,
	}
}

// RunPipeChainTest validates pipe chain validation logic.
func (pt *PipelineTester) RunPipeChainTest() *TestResult {
	start := time.Now()
	checker := NewPipeCompatibilityChecker()

	// Valid chain: image.resize (FILE output) -> hello.say (STRING input) -> ...
	// Actually this doesn't work per matrix. Let's test a valid one:
	// Stage 1: TEXT output -> Stage 2: STRING input (valid)
	// Stage 2: TEXT output -> Stage 3: FILE input (valid with warning)

	validChain := [][2]string{
		{"TEXT", "STRING"},
		{"TEXT", "FILE"},
	}
	r1 := checker.ValidatePipeChain(validChain)
	if !r1.Valid {
		return &TestResult{
			Name:        "pipe_chain_valid",
			Description: "Pipe chain validation accepts valid chains",
			Status:      TestFailed,
			Message:     fmt.Sprintf("Valid pipe chain was rejected: %s", r1.Summary()),
			DurationMs:  time.Since(start).Milliseconds(),
		}
	}

	// Invalid chain: FILE output -> INT input (invalid)
	invalidChain := [][2]string{
		{"FILE", "INT"},
	}
	r2 := checker.ValidatePipeChain(invalidChain)
	if r2.Valid {
		return &TestResult{
			Name:        "pipe_chain_invalid",
			Description: "Pipe chain validation rejects invalid chains",
			Status:      TestFailed,
			Message:     "Invalid pipe chain was accepted",
			DurationMs:  time.Since(start).Milliseconds(),
		}
	}

	return &TestResult{
		Name:        "pipe_chain_test",
		Description: "Pipe chain validation correctly accepts/rejects",
		Status:      TestPassed,
		Message:     "Pipe chain validation works correctly",
		DurationMs:  time.Since(start).Milliseconds(),
	}
}
