package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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
  unicli run --image <ref> [-- <container-cmd>...]
  unicli registry list
  unicli registry install <dir>
  unicli registry inspect <name>
  unicli registry remove <name>
  unicli help

Examples:
  unicli run --image hello.say:latest -- /app/say.sh "你好郭总"
  unicli run --image alpine:3.20 -- echo "Hello from UniCLI OS"`)
}

func cmdRun(args []string) {
	var imageRef string
	var cmdArgs []string
	passedDash := false

	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			passedDash = true
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

	// Determine DOCKER_HOST
	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost == "" {
		home, _ := os.UserHomeDir()
		sock := filepath.Join(home, ".docker", "run", "docker.sock")
		if _, err := os.Stat(sock); err == nil {
			dockerHost = "unix://" + sock
		}
	}

	// Build docker run command: docker run [OPTIONS] IMAGE [CMD...]
	dockerArgs := []string{"run", "--rm", "-i"}
	dockerArgs = append(dockerArgs, imageRef)
	if len(cmdArgs) > 0 {
		dockerArgs = append(dockerArgs, cmdArgs...)
	}
	if passedDash && len(cmdArgs) == 0 {
		// User passed -- but no args, just run default entrypoint
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
				fmt.Println("No skills installed in registry.")
				return
			}
			fmt.Fprintf(os.Stderr, "Error reading registry: %v\n", err)
			os.Exit(1)
		}
		var skills []string
		for _, e := range entries {
			if e.IsDir() {
				skills = append(skills, e.Name())
			}
		}
		if len(skills) == 0 {
			fmt.Println("No skills installed in registry.")
			return
		}
		fmt.Printf("Installed skills (%d):\n", len(skills))
		for _, s := range skills {
			fmt.Printf("  - %s\n", s)
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

		fmt.Printf("Installed '%s' from %s\n", name, srcDir)

	case "inspect":
		if len(subargs) < 1 {
			fmt.Fprintln(os.Stderr, "Usage: unicli registry inspect <name>")
			os.Exit(1)
		}
		name := subargs[0]
		skillDir := filepath.Join(regDir, name)
		info, err := os.Stat(skillDir)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Skill '%s' not found in registry\n", name)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if !info.IsDir() {
			fmt.Fprintf(os.Stderr, "Error: '%s' is not a valid skill\n", name)
			os.Exit(1)
		}

		hasManifest := false
		hasDockerfile := false
		entries, _ := os.ReadDir(skillDir)
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".cpl.json") {
				hasManifest = true
			}
			if e.Name() == "Dockerfile" {
				hasDockerfile = true
			}
		}

		fmt.Printf("Skill: %s\n", name)
		fmt.Printf("  Path: %s\n", skillDir)
		fmt.Printf("  Manifest: %v\n", hasManifest)
		fmt.Printf("  Dockerfile: %v\n", hasDockerfile)

		// Check if image is built
		dockerHost := os.Getenv("DOCKER_HOST")
		if dockerHost == "" {
			home, _ := os.UserHomeDir()
			sock := filepath.Join(home, ".docker", "run", "docker.sock")
			if _, err := os.Stat(sock); err == nil {
				dockerHost = "unix://" + sock
			}
		}
		imgCmd := exec.Command("docker", "images", "--format", "{{.Repository}}", name)
		if dockerHost != "" {
			imgCmd.Env = append(os.Environ(), "DOCKER_HOST="+dockerHost)
		}
		output, _ := imgCmd.Output()
		imageTag := strings.TrimSpace(string(output))
		if imageTag != "" {
			fmt.Printf("  Image: %s\n", imageTag)
		} else {
			fmt.Printf("  Image: (not built)\n")
		}

	case "remove":
		if len(subargs) < 1 {
			fmt.Fprintln(os.Stderr, "Usage: unicli registry remove <name>")
			os.Exit(1)
		}
		name := subargs[0]
		skillDir := filepath.Join(regDir, name)
		if err := os.RemoveAll(skillDir); err != nil {
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
