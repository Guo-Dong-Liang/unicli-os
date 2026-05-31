package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	case "init":
		cmdInit(args)
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
  unicli init [tool-name]                 Scaffold a new tool (interactive)
  unicli registry list                    List installed tools
  unicli registry install <dir>           Install a tool from directory
  unicli registry inspect <name>          Inspect an installed tool
  unicli registry remove <name>           Remove an installed tool
  unicli help                             Show this help

Examples:
  unicli run hello.say --name 果果
  unicli init my-tool                      # Create a new tool
  unicli registry install ./my-tool/       # Install it
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

	// Check for pipe mode
	quiet := false
	var filteredArgs []string
	for i := 0; i < len(toolArgs); i++ {
		if toolArgs[i] == "--quiet" || toolArgs[i] == "-q" {
			quiet = true
			continue
		}
		filteredArgs = append(filteredArgs, toolArgs[i])
	}
	toolArgs = filteredArgs

	// Auto-detect pipe: if stdout is not a terminal AND stdin is piped
	statOut, _ := os.Stdout.Stat()
	statIn, _ := os.Stdin.Stat()
	if (statOut.Mode()&os.ModeCharDevice) == 0 || (statIn.Mode()&os.ModeCharDevice) == 0 {
		quiet = true
	}

	// Find manifest in registry
	regDir := getRegistryDir()
	manifestPath := filepath.Join(regDir, toolName, toolName+".cpl.json")

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		// Tool not found locally — try remote registry
		if !quiet {
			fmt.Fprintf(os.Stderr, "🔍 Tool '%s' not found locally, searching remote...\n", toolName)
		}
		if remoteInstall(toolName) {
			data, err = os.ReadFile(manifestPath)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: tool '%s' not found in local or remote registry.\n", toolName)
			fmt.Fprintf(os.Stderr, "  Search: unicli registry search %s\n", toolName)
			os.Exit(1)
		}
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
	if strings.HasPrefix(entrypoint, "/") {
		// Container-style path: try basename first
		entrypoint = filepath.Join(toolDir, filepath.Base(entrypoint))
	} else if !filepath.IsAbs(entrypoint) {
		entrypoint = filepath.Join(toolDir, entrypoint)
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

	if !quiet {
		fmt.Fprintf(os.Stderr, "🔧 unicli run %s\n", toolName)
		fmt.Fprintf(os.Stderr, "   Entrypoint: %s\n", entrypoint)
	}

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

	case "remote":
		remoteURL := getRemoteURL()
		if len(subargs) >= 2 && subargs[0] == "set" {
			setRemoteURL(subargs[1])
			fmt.Printf("Remote registry set to: %s\n", subargs[1])
		} else {
			fmt.Printf("Remote registry: %s\n", remoteURL)
			fmt.Println("  Change: unicli registry remote set <url>")
		}

	case "search":
		query := ""
		if len(subargs) > 0 {
			query = strings.Join(subargs, " ")
		}
		searchRemote(query)

	case "login":
		if len(subargs) >= 2 && subargs[0] == "gitea" {
			setGiteaToken(subargs[1])
			fmt.Println("✅ Gitea token saved")
		} else {
			fmt.Println("Usage: unicli registry login gitea <token>")
			fmt.Println("  Get a token from: http://192.168.1.87:3000/user/settings/applications")
		}

	case "publish":
		toolDir := "."
		if len(subargs) > 0 {
			toolDir = subargs[0]
		}
		publishTool(toolDir)

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

// --- Init (scaffolding) ---

func cmdInit(args []string) {
	toolName := ""
	if len(args) > 0 {
		toolName = args[0]
	}

	reader := func(prompt string) string {
		fmt.Print(prompt)
		var input string
		fmt.Scanln(&input)
		return input
	}

	// 1. Tool name
	if toolName == "" {
		toolName = reader("  Tool name: ")
		for toolName == "" {
			toolName = reader("  Tool name (required): ")
		}
	}

	toolDir := toolName
	fmt.Printf("\n📦 Creating tool: %s\n\n", toolName)

	// 2. Description
	desc := reader(fmt.Sprintf("  Description (%s): ", "示例工具"))
	if desc == "" {
		desc = "示例工具"
	}

	// 3. Entrypoint type
	fmt.Println("\n  Entrypoint type:")
	fmt.Println("    1) Python (.py)")
	fmt.Println("    2) Shell (.sh)")
	epChoice := reader("  Choice [1]: ")
	if epChoice == "" {
		epChoice = "1"
	}

	epFile := "run.sh"
	template := ``
	if epChoice == "1" {
		epFile = "main.py"
		template = `#!/usr/bin/env python3
"""%s - %s"""
import argparse

def main():
    parser = argparse.ArgumentParser(description="%s")
    parser.add_argument('--input', help='输入参数')
    args = parser.parse_args()

    result = f"Hello from %s! input={args.input}"
    print(result)

if __name__ == '__main__':
    main()
`
	} else {
		template = `#!/bin/bash
# %s - %s
# Usage: ./run.sh --input VALUE

echo "Hello from %s! input=$1"
`
	}

	// 4. Inputs
	fmt.Println("\n  Define inputs (leave name empty to finish):")
	var inputs []struct {
		Name     string
		Flag     string
		Type     string
		Required bool
		Default  string
	}

	for i := 1; ; i++ {
		inName := reader(fmt.Sprintf("  Input %d name: ", i))
		if inName == "" {
			break
		}
		inFlag := reader(fmt.Sprintf("    --flag [--%s]: ", inName))
		if inFlag == "" {
			inFlag = "--" + inName
		}
		inType := reader(fmt.Sprintf("    type [STRING]: "))
		if inType == "" {
			inType = "STRING"
		}
		inReq := reader(fmt.Sprintf("    required? [y/N]: "))
		inputs = append(inputs, struct {
			Name     string
			Flag     string
			Type     string
			Required bool
			Default  string
		}{
			Name: inName,
			Flag: inFlag,
			Type: inType,
			Required: inReq == "y" || inReq == "Y",
		})

		fmt.Println()
		if len(inputs) >= 8 {
			break
		}
	}

	// 5. Create directory
	os.MkdirAll(toolDir, 0755)

	// Write entrypoint
	entryContent := fmt.Sprintf(template, toolName, desc, desc, toolName)
	os.WriteFile(filepath.Join(toolDir, epFile), []byte(entryContent), 0755)

	// Build manifest
	manifest := CPLManifest{
		CPLVersion:  "1.0.0",
		Name:        toolName,
		Version:     "1.0.0",
		Description: desc,
		Author:      "UniCLI User",
		Inputs:      make([]CPLInput, 0),
		Outputs: []CPLOutput{
			{Name: "output", Type: "TEXT", Description: "工具输出", CaptureStdout: true},
		},
		Resources: CPLResource{CPU: 1, Memory: 256, Network: false, GPU: false, Timeout: 60, Disk: 128},
		Image:     CPLImage{Ref: fmt.Sprintf("ghcr.io/unixcli/%s:1.0.0", toolName), Entrypoint: epFile, Workdir: "/workspace", User: "nobody:nogroup"},
	}

	for _, in := range inputs {
		inp := CPLInput{
			Name:        in.Name,
			Type:        in.Type,
			Required:    in.Required,
			Flag:        in.Flag,
			Description: fmt.Sprintf("%s 参数", in.Name),
		}
		if in.Default != "" {
			inp.Default = in.Default
		}
		manifest.Inputs = append(manifest.Inputs, inp)
	}

	// If no inputs defined, add a sample one
	if len(inputs) == 0 {
		manifest.Inputs = append(manifest.Inputs, CPLInput{
			Name: "input", Type: "STRING", Required: false,
			Flag: "--input", Description: "输入参数",
		})
	}

	// Write manifest
	manData, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(filepath.Join(toolDir, toolName+".cpl.json"), manData, 0644)

	// Summary
	fmt.Printf("\n✅ Created tool: %s/\n", toolDir)
	fmt.Printf("   ├── %s.cpl.json    (manifest)\n", toolName)
	fmt.Printf("   └── %s          (entrypoint)\n\n", epFile)
	fmt.Printf("  Next steps:\n")
	fmt.Printf("    cd %s\n", toolDir)
	fmt.Printf("    unicli registry install .\n")
	fmt.Printf("    unicli run %s --input test\n\n", toolName)
	}

	// --- Remote Registry ---

	type RegistryIndex struct {
	RegistryVersion string        `json:"registry_version"`
	BaseURL         string        `json:"base_url"`
	Tools           []RegistryTool `json:"tools"`
	}

	type RegistryTool struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Path        string `json:"path"`
	}

	func getUniclircPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".unicli", "config.json")
	}

	func getRemoteURL() string {
	defaultURL := "http://192.168.1.87:3000/admin/unicli-os/raw/main/registry/index.json"
	cfgPath := getUniclircPath()
	if data, err := os.ReadFile(cfgPath); err == nil {
	var cfg map[string]string
	if json.Unmarshal(data, &cfg) == nil {
		if url, ok := cfg["remote_registry"]; ok && url != "" {
			return url
		}
	}
	}
	return defaultURL
	}

	func setRemoteURL(url string) {
	cfgPath := getUniclircPath()
	os.MkdirAll(filepath.Dir(cfgPath), 0755)
	cfg := map[string]string{"remote_registry": url}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(cfgPath, data, 0644)
	}

	func fetchRemoteIndex() (*RegistryIndex, error) {
	url := getRemoteURL()
	resp, err := http.Get(url)
	if err != nil {
	return nil, fmt.Errorf("cannot connect to remote registry: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
	return nil, fmt.Errorf("remote registry returned HTTP %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var idx RegistryIndex
	if err := json.Unmarshal(body, &idx); err != nil {
	return nil, fmt.Errorf("invalid registry index: %v", err)
	}
	return &idx, nil
	}

	func searchRemote(query string) {
	idx, err := fetchRemoteIndex()
	if err != nil {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
	}

	fmt.Printf("Remote registry: %s\n", idx.BaseURL)
	fmt.Printf("Available tools (%d):\n\n", len(idx.Tools))

	query = strings.ToLower(query)
	found := 0
	for _, t := range idx.Tools {
	if query != "" {
		nameMatch := strings.Contains(strings.ToLower(t.Name), query)
		descMatch := strings.Contains(strings.ToLower(t.Description), query)
		if !nameMatch && !descMatch {
			continue
		}
	}
	fmt.Printf("  📦 %s  v%s\n", t.Name, t.Version)
	fmt.Printf("     %s\n", t.Description)
	fmt.Printf("     Install: unicli registry install %s\n", t.Name)
	fmt.Println()
	found++
	}
	if query != "" {
	fmt.Printf("Found %d tool(s) matching '%s'\n", found, query)
	}
	}

	func remoteInstall(toolName string) bool {
	idx, err := fetchRemoteIndex()
	if err != nil {
	fmt.Fprintf(os.Stderr, "  Remote unavailable: %v\n", err)
	return false
	}

	// Find tool in index
	var toolInfo *RegistryTool
	for _, t := range idx.Tools {
	if t.Name == toolName {
		toolInfo = &t
		break
	}
	}
	if toolInfo == nil {
	return false
	}

	// Download tool files from remote
	baseURL := strings.TrimSuffix(idx.BaseURL, "/index.json")
	regDir := getRegistryDir()
	toolDir := filepath.Join(regDir, toolName)

	// Fetch file list from the remote tool directory
	// We know the structure: <path>/<name>.cpl.json + entrypoint file(s)
	fmt.Fprintf(os.Stderr, "  📥 Downloading '%s' from remote...\n", toolName)
	os.MkdirAll(toolDir, 0755)

	filesToFetch := []string{
	toolInfo.Path + "/" + toolName + ".cpl.json",
	}

	// Try to fetch common entrypoint files
	for _, entry := range []string{".sh", ".py", ".bat"} {
	filesToFetch = append(filesToFetch, toolInfo.Path+"/main"+entry)
	filesToFetch = append(filesToFetch, toolInfo.Path+"/run"+entry)
	}

	downloaded := 0
	for _, filePath := range filesToFetch {
	fileURL := baseURL + "/" + filePath
	resp, err := http.Get(fileURL)
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			resp.Body.Close()
		}
		continue
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	localName := filepath.Base(filePath)
	os.WriteFile(filepath.Join(toolDir, localName), body, 0755)
	downloaded++
	}

	if downloaded == 0 {
	// Fallback: try listing files from the tool's Gitea tree API
	fmt.Fprintf(os.Stderr, "  ⚠ Could not download tool files.\n")
	os.RemoveAll(toolDir)
	return false
	}

	fmt.Fprintf(os.Stderr, "  ✅ Installed '%s' (%d files)\n", toolName, downloaded)
	fmt.Fprintf(os.Stderr, "  ▶  Run: unicli run %s\n", toolName)
	return true
	}
