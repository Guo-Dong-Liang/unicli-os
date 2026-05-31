package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// --- CPL Manifest types ---

type CPLManifest struct {
	CPLVersion  string      `json:"cpl_version"`
	Name        string      `json:"name"`
	Version     string      `json:"version"`
	Description string      `json:"description"`
	Author      string      `json:"author"`
	Inputs      []CPLInput  `json:"inputs"`
	Outputs     []CPLOutput `json:"outputs"`
	Resources   CPLResource `json:"resources"`
	Image       CPLImage    `json:"image"`
}

type CPLInput struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default"`
	Description string      `json:"description"`
	Flag        string      `json:"flag"`
	Position    int         `json:"position"`
}

type CPLOutput struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	Description    string `json:"description"`
	CaptureStdout  bool   `json:"capture_stdout"`
}

type CPLResource struct {
	CPU     float64 `json:"cpu"`
	Memory  int     `json:"memory"`
	Network bool    `json:"network"`
	GPU     bool    `json:"gpu"`
	Timeout int     `json:"timeout"`
	Disk    int     `json:"disk"`
}

type CPLImage struct {
	Ref        string `json:"ref"`
	Entrypoint string `json:"entrypoint"`
	Workdir    string `json:"workdir"`
	User       string `json:"user"`
}

// --- Main ---

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "run":
		cmdRun(args)
	case "registry":
		cmdRegistry(args)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`UniCLI OS — Universal Containerized Language Interface

Usage:
  unicli run <tool-name> [flags...]       Run a tool from registry (local)
  unicli run --image <ref> [-- <cmd>...]  Run a Docker image
  unicli registry list                    List installed tools
  unicli registry install <dir>           Install a tool from directory
  unicli registry inspect <name>          Inspect an installed tool
  unicli registry remove <name>           Remove an installed tool
  unicli help                             Show this help

Examples:
  unicli run hello.say --name 果果
  unicli run --image alpine:3.20 -- echo "Hello"
  unicli registry install ./examples/hello.say/`)
}

// --- Run ---

func cmdRun(args []string) {
	// If first arg is --image, use Docker mode (original behavior)
	if len(args) > 0 && args[0] == "--image" {
		cmdRunDocker(args)
		return
	}

	// Otherwise, treat first arg as tool name -> local run mode
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Error: specify a tool name or --image")
		os.Exit(1)
	}

	cmdRunLocal(args)
}

// --- Local Run Mode (Phase 1) ---

func cmdRunLocal(args []string) {
	toolName := args[0]
	toolArgs := args[1:]

	// Find manifest in registry
	regDir := getRegistryDir()
	manifestPath := filepath.Join(regDir, toolName, toolName+".cpl.json")

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: tool '%s' not found in registry.\n", toolName)
		fmt.Fprintf(os.Stderr, "  Looked in: %s\n", manifestPath)
		fmt.Fprintf(os.Stderr, "  Install it first: unicli registry install <path-to-tool-dir>\n")
		os.Exit(1)
	}

	var manifest CPLManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid manifest: %v\n", err)
		os.Exit(1)
	}

	// Determine entrypoint path
	toolDir := filepath.Join(regDir, toolName)
	entrypoint := manifest.Image.Entrypoint

	// Resolve entrypoint path relative to tool directory
	if !filepath.IsAbs(entrypoint) {
		entrypoint = filepath.Join(toolDir, entrypoint)
	} else if strings.HasPrefix(entrypoint, "/") {
		// Container-style path, try to find locally
		rel := strings.TrimPrefix(entrypoint, "/")
		entrypoint = filepath.Join(toolDir, rel)
		// If not found, try just the basename
		if _, err := os.Stat(entrypoint); os.IsNotExist(err) {
			entrypoint = filepath.Join(toolDir, filepath.Base(entrypoint))
		}
	}

	// Check entrypoint exists
	if _, err := os.Stat(entrypoint); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: entrypoint not found: %s\n", entrypoint)
		os.Exit(1)
	}

	// Parse tool args against manifest inputs
	cmdArgs := buildCommandArgs(manifest, toolArgs)

	// Determine how to run: shell scripts with bash, python with python
	var cmd *exec.Cmd
	if strings.HasSuffix(entrypoint, ".sh") {
		cmd = exec.Command("bash", append([]string{entrypoint}, cmdArgs...)...)
	} else if strings.HasSuffix(entrypoint, ".py") {
		// Try Hermes venv Python first, then fallback to system python
		pyPath := "C:\\Users\\Administrator\\AppData\\Local\\hermes\\hermes-agent\\venv\\Scripts\\python.exe"
		if _, err := os.Stat(pyPath); os.IsNotExist(err) {
			pyPath = "python3"
			if _, err2 := exec.LookPath("python3"); err2 != nil {
				pyPath = "python"
			}
		}
		cmd = exec.Command(pyPath, append([]string{entrypoint}, cmdArgs...)...)
	} else {
		// Make executable and run directly
		cmd = exec.Command(entrypoint, cmdArgs...)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = toolDir

	fmt.Fprintf(os.Stderr, "🔧 unicli run %s\n", toolName)
	fmt.Fprintf(os.Stderr, "   Entrypoint: %s\n", entrypoint)

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// buildCommandArgs maps CLI flags to manifest input definitions
func buildCommandArgs(manifest CPLManifest, cliArgs []string) []string {
	// Parse CLI args into a map
	argMap := make(map[string]string)
	var positional []string

	for i := 0; i < len(cliArgs); i++ {
		if strings.HasPrefix(cliArgs[i], "--") {
			flag := cliArgs[i]
			if i+1 < len(cliArgs) && !strings.HasPrefix(cliArgs[i+1], "--") {
				argMap[flag] = cliArgs[i+1]
				i++
			} else {
				argMap[flag] = "true"
			}
		} else {
			positional = append(positional, cliArgs[i])
		}
	}

	// Build command args based on manifest inputs
	var cmdArgs []string
	for _, input := range manifest.Inputs {
		flag := input.Flag
		if flag == "" {
			continue
		}

		// Check if user provided this flag
		if val, ok := argMap[flag]; ok {
			cmdArgs = append(cmdArgs, flag, val)
		} else if input.Required {
			// Required input not provided — skip, let the tool handle validation
			continue
		}
		// If not required and not provided, skip (use default)
	}

	// Append remaining unmatched args
	for _, p := range positional {
		cmdArgs = append(cmdArgs, p)
	}

	return cmdArgs
}

// --- Docker Run Mode (original) ---

func cmdRunDocker(args []string) {
	var imageRef string
	var cmdArgs []string

	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			cmdArgs = args[i+1:]
			break
		}
		if args[i] == "--image" && i+1 < len(args) {
			imageRef = args[i+1]
			i++
		}
	}

	if imageRef == "" {
		fmt.Fprintln(os.Stderr, "Error: --image is required")
		os.Exit(1)
	}

	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost == "" {
		home, _ := os.UserHomeDir()
		sock := filepath.Join(home, ".docker", "run", "docker.sock")
		if _, err := os.Stat(sock); err == nil {
			dockerHost = "unix://" + sock
		}
	}

	dockerArgs := []string{"run", "--rm", "-i"}
	dockerArgs = append(dockerArgs, imageRef)
	if len(cmdArgs) > 0 {
		dockerArgs = append(dockerArgs, cmdArgs...)
	}

	cmd := exec.Command("docker", dockerArgs...)
	if dockerHost != "" {
		cmd.Env = append(os.Environ(), "DOCKER_HOST="+dockerHost)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// --- Registry ---

func getRegistryDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".unicli", "registry")
	}
	return filepath.Join(home, ".unicli", "registry")
}

func cmdRegistry(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: unicli registry <list|install|inspect|remove>")
		os.Exit(1)
	}

	regDir := getRegistryDir()
	subcmd := args[0]
	subargs := args[1:]

	switch subcmd {
	case "list":
		entries, err := os.ReadDir(regDir)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No tools installed in registry.")
				return
			}
			fmt.Fprintf(os.Stderr, "Error reading registry: %v\n", err)
			os.Exit(1)
		}
		var tools []string
		for _, e := range entries {
			if e.IsDir() {
				tools = append(tools, e.Name())
			}
		}
		if len(tools) == 0 {
			fmt.Println("No tools installed in registry.")
			return
		}
		fmt.Printf("Installed tools (%d):\n", len(tools))
		for _, t := range tools {
			// Read version from manifest
			manifestPath := filepath.Join(regDir, t, t+".cpl.json")
			desc := ""
			if data, err := os.ReadFile(manifestPath); err == nil {
				var m CPLManifest
				if json.Unmarshal(data, &m) == nil {
					desc = m.Description
				}
			}
			fmt.Printf("  - %s", t)
			if desc != "" {
				fmt.Printf(": %s", desc)
			}
			fmt.Println()
		}

	case "install":
		if len(subargs) < 1 {
			fmt.Fprintln(os.Stderr, "Usage: unicli registry install <dir>")
			os.Exit(1)
		}
		srcDir := expandPath(subargs[0])
		info, err := os.Stat(srcDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if !info.IsDir() {
			fmt.Fprintf(os.Stderr, "Error: %s is not a directory\n", srcDir)
			os.Exit(1)
		}

		name := filepath.Base(srcDir)
		dstDir := filepath.Join(regDir, name)

		if err := os.MkdirAll(regDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating registry: %v\n", err)
			os.Exit(1)
		}

		os.RemoveAll(dstDir)
		copyCmd := exec.Command("cp", "-R", srcDir, dstDir)
		if output, err := copyCmd.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "Error installing: %v\n%s", err, output)
			os.Exit(1)
		}

		fmt.Printf("✅ Installed '%s' from %s\n", name, srcDir)
		fmt.Printf("   Run: unicli run %s\n", name)

	case "inspect":
		if len(subargs) < 1 {
			fmt.Fprintln(os.Stderr, "Usage: unicli registry inspect <name>")
			os.Exit(1)
		}
		name := subargs[0]
		toolDir := filepath.Join(regDir, name)
		info, err := os.Stat(toolDir)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Tool '%s' not found in registry\n", name)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if !info.IsDir() {
			fmt.Fprintf(os.Stderr, "Error: '%s' is not a valid tool\n", name)
			os.Exit(1)
		}

		manifestPath := filepath.Join(toolDir, name+".cpl.json")
		fmt.Printf("Tool: %s\n", name)
		fmt.Printf("  Path: %s\n", toolDir)
		if data, err := os.ReadFile(manifestPath); err == nil {
			var m CPLManifest
			if json.Unmarshal(data, &m) == nil {
				fmt.Printf("  Version: %s\n", m.Version)
				fmt.Printf("  Description: %s\n", m.Description)
				fmt.Printf("  Author: %s\n", m.Author)
				fmt.Printf("  Entrypoint: %s\n", m.Image.Entrypoint)
				fmt.Printf("  Inputs:\n")
				for _, inp := range m.Inputs {
					req := ""
					if inp.Required {
						req = " (required)"
					}
					fmt.Printf("    - %s (%s)%s: %s\n", inp.Name, inp.Type, req, inp.Description)
				}
			}
		}

	case "remove":
		if len(subargs) < 1 {
			fmt.Fprintln(os.Stderr, "Usage: unicli registry remove <name>")
			os.Exit(1)
		}
		name := subargs[0]
		toolDir := filepath.Join(regDir, name)
		if err := os.RemoveAll(toolDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error removing '%s': %v\n", name, err)
			os.Exit(1)
		}
		fmt.Printf("Removed '%s'\n", name)

	default:
		fmt.Printf("Unknown registry subcommand: %s\n", subcmd)
		os.Exit(1)
	}
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
