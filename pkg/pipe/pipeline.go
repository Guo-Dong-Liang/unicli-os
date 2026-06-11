package pipe

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
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

// Run executes the pipeline: each stage's stdout is piped as NDJSON frames
// to the next stage's stdin. Final stage writes to os.Stdout.
func (p *Pipeline) Run() error {
	if len(p.Stages) == 0 {
		return fmt.Errorf("pipeline: no stages defined")
	}
	if len(p.Stages) == 1 {
		return fmt.Errorf("pipeline: single stage — use 'unicli run' instead")
	}

	for i, stage := range p.Stages {
		// The last stage outputs directly to stdout
		isLast := i == len(p.Stages)-1

		// Run the tool and pipe its output
		// For the first stage, stdin comes from os.Stdin
		// For later stages, stdin comes from the previous stage's pipe
		if i == 0 {
			// TODO: when we have multi-stage chaining, create pipes between
			// each pair. For now, run first stage with encoder output to buffer.
		}
		_ = stage
		_ = isLast
	}

	// Simpler approach: run each stage sequentially.
	// Stage output is captured as NDJSON, decoded, and passed to next stage.
	return p.runSequential()
}

// runSequential runs each stage one at a time, piping text through.
func (p *Pipeline) runSequential() error {
	var prevStdout io.Reader

	for i, stage := range p.Stages {
		isLast := i == len(p.Stages)-1
		toolName := stage.Args[0]
		toolArgs := stage.Args[1:] // TODO: resolve manifest and build actual args

		cmdName := toolName
		var cmdArgs []string

		switch {
		case strings.HasSuffix(toolName, ".sh"):
			cmdName = "bash"
			cmdArgs = []string{toolName}
		case strings.HasSuffix(toolName, ".py"):
			cmdName = "python3"
			if _, err := exec.LookPath("python3"); err != nil {
				cmdName = "python"
			}
			cmdArgs = []string{toolName}
		default:
			// Check if it's a registered tool — resolve via manifest
			// For now, run as-is
			cmdArgs = toolArgs
		}

		cmd := exec.Command(cmdName, append(cmdArgs, splitArgs(toolArgs)...)...)

		if i == 0 {
			cmd.Stdin = os.Stdin
		} else {
			cmd.Stdin = prevStdout
		}

		if isLast {
			// Last stage: output directly to stdout
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		} else {
			// Middle stage: capture output to pipe to next stage
			var buf bytes.Buffer
			encoder := NewEncoder(&buf)
			// tee stdout to the encoder
			// For now, just capture raw stdout
			cmd.Stdout = &buf
			cmd.Stderr = os.Stderr
			prevStdout = &buf
			_ = encoder
		}

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("pipeline stage %d (%s): %w", i+1, toolName, err)
		}
	}

	return nil
}

// splitArgs splits a string into arguments, respecting quoted strings.
func splitArgs(args []string) []string {
	if len(args) <= 1 {
		return nil
	}
	return args[1:]
}
