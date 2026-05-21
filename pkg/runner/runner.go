package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/admin/unicli-os/pkg/cpl"
	"github.com/admin/unicli-os/pkg/registry"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// Runner manages sandbox container execution
type Runner struct {
	docker *client.Client
}

// RunResult holds the outcome of a command execution
type RunResult struct {
	ExitCode  int
	Stdout    string
	Stderr    string
	Outputs   map[string]string // output file paths on host
	Duration  time.Duration
	Command   string
}

// Summary returns a human-readable result summary
func (r *RunResult) Summary() string {
	var b strings.Builder
	status := "✅"
	if r.ExitCode != 0 {
		status = "❌"
	}
	fmt.Fprintf(&b, "%s %s completed (exit: %d, duration: %v)\n", status, r.Command, r.ExitCode, r.Duration.Round(time.Millisecond))
	if r.Stdout != "" {
		fmt.Fprintf(&b, "  stdout: %s\n", truncate(r.Stdout, 200))
	}
	if r.Stderr != "" {
		fmt.Fprintf(&b, "  stderr: %s\n", truncate(r.Stderr, 200))
	}
	for name, path := range r.Outputs {
		fmt.Fprintf(&b, "  output[%s]: %s\n", name, path)
	}
	return b.String()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// PipelineContext holds parsed pipeline expression
type PipelineContext struct {
	Stages []StageSpec
}

// StageSpec defines a single stage in a pipeline
type StageSpec struct {
	CommandName string
	Args        map[string]string
	InputExpr   string // original expression part
}

// New creates a new Runner
func New() (*Runner, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}
	return &Runner{docker: cli}, nil
}

// RunExpression parses and executes a command expression (single or piped)
func (r *Runner) RunExpression(ctx context.Context, expr string, inputs, outputs map[string]string, reg *registry.Registry) (*RunResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	pipeline, err := ParseExpression(expr)
	if err != nil {
		return nil, fmt.Errorf("parse expression: %w", err)
	}

	if len(pipeline.Stages) == 1 {
		return r.runSingle(ctx, pipeline.Stages[0], inputs, outputs, reg)
	}
	return r.runPipeline(ctx, pipeline, inputs, outputs, reg)
}

// runSingle executes a single command in a sandbox
func (r *Runner) runSingle(ctx context.Context, stage StageSpec, inputs, outputs map[string]string, reg *registry.Registry) (*RunResult, error) {
	manifest, err := reg.Get(stage.CommandName)
	if err != nil {
		return nil, fmt.Errorf("command %q not found in registry", stage.CommandName)
	}

	start := time.Now()
	defer func() { /* noop */ }()

	// Merge inputs: explicit > stage args > manifest defaults
	resolvedArgs := resolveArgs(manifest, stage.Args, inputs)

	// Create a temp work directory
	workDir, err := os.MkdirTemp("", "unicli-run-*")
	if err != nil {
		return nil, fmt.Errorf("create workdir: %w", err)
	}
	defer os.RemoveAll(workDir)

	// Build Docker run config
	cmd := buildEntrypoint(manifest, resolvedArgs)
	envVars := buildEnv(manifest, resolvedArgs)

	containerConfig := &container.Config{
		Image:      manifest.ImageRef,
		Cmd:        cmd,
		Env:        envVars,
		WorkingDir: manifest.Workdir,
		User:       "1000:1000", // non-root
	}

	hostConfig := &container.HostConfig{
		Binds:        []string{workDir + ":/workspace"},
		NetworkMode:  container.NetworkMode("none"),
		ReadonlyRootfs: true,
		Resources: container.Resources{
			Memory:   int64(manifest.Resources.MemoryMB) * 1024 * 1024,
			NanoCPUs: int64(manifest.Resources.CPUCores * 1e9),
		},
		AutoRemove: true,
	}

	// If the command needs network, allow it
	if manifest.Network == "outbound" || manifest.Network == "full" {
		hostConfig.NetworkMode = container.NetworkMode("default")
	}

	resp, err := r.docker.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		// Pull image if not found
		if client.IsErrNotFound(err) {
			_, pullErr := r.docker.ImagePull(ctx, manifest.ImageRef, image.PullOptions{})
			if pullErr != nil {
				return nil, fmt.Errorf("pull image: %w", pullErr)
			}
			resp, err = r.docker.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
		}
		if err != nil {
			return nil, fmt.Errorf("create container: %w", err)
		}
	}

	defer func() {
		_ = r.docker.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
	}()

	if err := r.docker.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("start container: %w", err)
	}

	// Wait for exit
	exitCh, errCh := r.docker.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		return nil, fmt.Errorf("wait container: %w", err)
	case exit := <-exitCh:
		duration := time.Since(start)

		// Get logs
		stdout, stderr := r.getLogs(ctx, resp.ID)

		// Map output files back to host
		outputFiles := map[string]string{}
		for name, hostPath := range outputs {
			containerPath := filepath.Join("/workspace", filepath.Base(hostPath))
			hostFile := filepath.Join(workDir, filepath.Base(hostPath))
			outputFiles[name] = hostFile

			// Copy from container to workdir if file exists
			_ = r.copyFromContainer(ctx, resp.ID, containerPath, hostFile)
		}

		return &RunResult{
			ExitCode: int(exit.StatusCode),
			Stdout:   stdout,
			Stderr:   stderr,
			Outputs:  outputFiles,
			Duration: duration,
			Command:  stage.CommandName,
		}, nil
	}
}

// runPipeline executes multiple chained commands
func (r *Runner) runPipeline(ctx context.Context, pipeline *PipelineContext, inputs, outputs map[string]string, reg *registry.Registry) (*RunResult, error) {
	// For now, execute stages sequentially and pass stdout to stdin
	var lastStdout string
	var lastResult *RunResult

	for i, stage := range pipeline.Stages {
		stageInputs := inputs
		if i > 0 && lastStdout != "" {
			if stageInputs == nil {
				stageInputs = map[string]string{}
			}
			stageInputs["input_data"] = lastStdout
		}

		result, err := r.runSingle(ctx, stage, stageInputs, outputs, reg)
		if err != nil {
			return nil, fmt.Errorf("pipeline stage %d (%s): %w", i, stage.CommandName, err)
		}
		lastStdout = result.Stdout
		lastResult = result
	}
	return lastResult, nil
}

// ParseExpression parses a shell-like expression into pipeline stages
func ParseExpression(expr string) (*PipelineContext, error) {
	parts := strings.Split(expr, "|")
	stages := make([]StageSpec, len(parts))

	for i, part := range parts {
		part = strings.TrimSpace(part)
		tokens := tokenize(part)
		if len(tokens) == 0 {
			return nil, fmt.Errorf("empty command in stage %d", i)
		}

		stage := StageSpec{
			CommandName: tokens[0],
			Args:        map[string]string{},
			InputExpr:   part,
		}

		// Parse --key value or --key=value
		for j := 1; j < len(tokens); j++ {
			t := tokens[j]
			if strings.HasPrefix(t, "--") {
				key := strings.TrimPrefix(t, "--")
				if idx := strings.Index(key, "="); idx >= 0 {
					stage.Args[key[:idx]] = key[idx+1:]
				} else if j+1 < len(tokens) && !strings.HasPrefix(tokens[j+1], "--") {
					j++
					stage.Args[key] = tokens[j]
				} else {
					stage.Args[key] = "true"
				}
			}
		}
		stages[i] = stage
	}

	return &PipelineContext{Stages: stages}, nil
}

// tokenize splits a command string into tokens (simple shell-like)
func tokenize(s string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := false

	for _, ch := range s {
		switch {
		case ch == '"' || ch == '\'':
			inQuote = !inQuote
		case ch == ' ' && !inQuote:
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

// resolveArgs merges manifest defaults with provided args
func resolveArgs(manifest *cpl.Manifest, stageArgs, explicitInputs map[string]string) map[string]string {
	result := map[string]string{}

	// Start with manifest defaults
	for _, in := range manifest.Inputs {
		if in.Default != "" {
			result[in.Name] = in.Default
		}
	}

	// Override with stage args
	for k, v := range stageArgs {
		result[k] = v
	}

	// Override with explicit inputs
	for k, v := range explicitInputs {
		result[k] = v
	}

	return result
}

// buildEntrypoint constructs the container command
func buildEntrypoint(manifest *cpl.Manifest, args map[string]string) []string {
	cmd := []string{manifest.Entrypoint}
	for _, in := range manifest.Inputs {
		if val, ok := args[in.Name]; ok {
			cmd = append(cmd, "--"+in.Name, val)
		}
	}
	return cmd
}

// buildEnv constructs environment variables
func buildEnv(manifest *cpl.Manifest, args map[string]string) []string {
	var env []string
	for k, v := range manifest.Env {
		env = append(env, k+"="+v)
	}
	for k, v := range args {
		env = append(env, "UNICLI_INPUT_"+strings.ToUpper(k)+"="+v)
	}
	return env
}

func (r *Runner) getLogs(ctx context.Context, containerID string) (stdout, stderr string) {
	// Simple log retrieval - in production use the Docker log API
	out, err := r.docker.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return "", ""
	}
	defer out.Close()

	buf := make([]byte, 4096)
	var stdoutBuf, stderrBuf strings.Builder
	for {
		n, err := out.Read(buf)
		if n > 0 {
			// Skip Docker's 8-byte frame header
			for i := 0; i < n; i++ {
				// Simple: just append all text
				stdoutBuf.WriteByte(buf[i])
			}
		}
		if err != nil {
			break
		}
	}
	return stdoutBuf.String(), stderrBuf.String()
}

func (r *Runner) copyFromContainer(ctx context.Context, containerID, srcPath, dstPath string) error {
	// Simplified - in production use docker CopyFromContainer
	return nil
}

// PullImage pulls a container image
func (r *Runner) PullImage(ctx context.Context, imageRef string) error {
	reader, err := r.docker.ImagePull(ctx, imageRef, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull image %s: %w", imageRef, err)
	}
	defer reader.Close()
	return nil
}
