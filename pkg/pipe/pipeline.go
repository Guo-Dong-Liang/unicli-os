package pipe

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/unixcli/unicli-os/pkg/cpl"
)

// Stage describes one step in a pipeline.
type Stage struct {
	Manifest *cpl.CPLManifest
	Args     []string
}

// Pipeline chains multiple tool stages, piping stdout of each to the stdin
// of the next using the NDJSON frame protocol.
type Pipeline struct {
	Stages []Stage
	Quiet  bool
}

// ParsePipeline parses a pipeline expression like "tool1 --flag val | tool2 --other".
// It splits on `|` boundaries and returns a Pipeline with the correct stages.
func ParsePipeline(expr string) *Pipeline {
	p := &Pipeline{}
	parts := strings.Split(expr, "|")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		tokens := strings.Fields(part)
		if len(tokens) == 0 {
			continue
		}
		p.Stages = append(p.Stages, Stage{
			Args: tokens,
		})
	}
	return p
}

// Run executes the pipeline: each stage's raw stdout is piped as text
// to the next stage's stdin. The final stage writes to os.Stdout.
func (p *Pipeline) Run() error {
	if len(p.Stages) == 0 {
		return fmt.Errorf("pipeline: no stages defined")
	}
	if len(p.Stages) == 1 {
		return fmt.Errorf("pipeline: single stage — use 'unicli run' instead")
	}
	return p.runSequential()
}

// runSequential runs each stage one at a time, piping stdout to stdin.
func (p *Pipeline) runSequential() error {
	var prevStdout io.Reader

	for i, stage := range p.Stages {
		isLast := i == len(p.Stages)-1
		toolName := stage.Args[0]
		stageArgs := stage.Args[1:]

		// Resolve command and args for this stage
		cmdName, cmdArgs := resolveCommand(toolName, stageArgs)

		cmd := exec.Command(cmdName, cmdArgs...)

		if i == 0 {
			cmd.Stdin = os.Stdin
		} else {
			cmd.Stdin = prevStdout
		}

		if isLast {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		} else {
			// Capture output to pipe to next stage as raw text
			var buf bytes.Buffer
			cmd.Stdout = &buf
			cmd.Stderr = os.Stderr
			prevStdout = &buf
		}

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("pipeline stage %d (%s): %w", i+1, toolName, err)
		}

		// If this stage is not the last, prep the encoder for NDJSON framing
		// for the next stage (future: structured pipe protocol)
		if !isLast {
			// For MVP: pass raw stdout directly (already in buffer)
			// Upgrade to NDJSON framing later via Encoder
		}
	}

	return nil
}

// resolveCommand determines the OS command to run for a tool name.
func resolveCommand(toolName string, args []string) (string, []string) {
	// If it's a file path (.sh / .py), use the appropriate interpreter
	switch {
	case strings.HasSuffix(toolName, ".sh"):
		return "bash", append([]string{toolName}, args...)
	case strings.HasSuffix(toolName, ".py"):
		py := "python3"
		if _, err := exec.LookPath("python3"); err != nil {
			py = "python"
		}
		return py, append([]string{toolName}, args...)
	default:
		// Check if it's a registered tool — resolve via registry manifest
		// For now, try as an absolute path or PATH lookup
		if filepath.IsAbs(toolName) || strings.Contains(toolName, "/") || strings.Contains(toolName, "\\") {
			return toolName, args
		}
		// It's a tool name, not a file path — pass as command with args
		return toolName, args
	}
}
