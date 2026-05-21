package main

import (
	"fmt"
	"os"

	"github.com/admin/unicli-os/pkg/cpl"
	"github.com/admin/unicli-os/pkg/registry"
	"github.com/admin/unicli-os/pkg/runner"
	"github.com/spf13/cobra"
)

var Version = "dev"
var Commit = "unknown"

func main() {
	var rootCmd = &cobra.Command{
		Use:   "unicli",
		Short: "UniCLI — Universal Command Line Interface OS",
		Long: `UniCLI is the next-generation OS for running CLI commands
as on-demand sandboxed containers. Run, chain, and manage commands
with full isolation and security.

  unicli run "image.resize --width 800" < input.jpg
  unicli run "image.resize | image.grayscale" --input photo.jpg
  unicli run hello.say --name "World"
  unicli registry list
  unicli inspect image.resize`,
		Version: fmt.Sprintf("%s (commit: %s)", Version, Commit),
	}

	// --- run subcommand ---
	var runInputs map[string]string
	var runOutputs map[string]string

	var runCmd = &cobra.Command{
		Use:   "run [command]",
		Short: "Run a CLI command",
		Long: `Run a UniCLI command in an isolated sandbox.
Commands can be chained with pipe (|) for structured data pipelines.

Examples:
  unicli run hello.say --name World
  unicli run "image.resize --width 800" --input input=photo.jpg --output output=resized.jpg
  unicli run "hello.say --name World | hello.say --greeting Hi"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			expr := args[0]
			r, err := runner.New()
			if err != nil {
				return fmt.Errorf("failed to create runner: %w", err)
			}
			reg, err := registry.OpenDefault()
			if err != nil {
				return fmt.Errorf("failed to open registry: %w", err)
			}

			result, err := r.RunExpression(cmd.Context(), expr, runInputs, runOutputs, reg)
			if err != nil {
				return fmt.Errorf("run failed: %w", err)
			}

			fmt.Println(result.Summary())
			return nil
		},
	}
	runCmd.Flags().StringToStringVar(&runInputs, "input", nil, "Input parameters (key=value)")
	runCmd.Flags().StringToStringVar(&runOutputs, "output", nil, "Output paths (key=path)")
	rootCmd.AddCommand(runCmd)

	// --- registry subcommand ---
	var registryCmd = &cobra.Command{
		Use:   "registry",
		Short: "Manage installed CLI commands",
	}
	rootCmd.AddCommand(registryCmd)

	var registryListCmd = &cobra.Command{
		Use:   "list",
		Short: "List all installed commands",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := registry.OpenDefault()
			if err != nil {
				return fmt.Errorf("failed to open registry: %w", err)
			}
			entries, err := reg.List()
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				fmt.Println("No commands installed.")
				return nil
			}
			for _, e := range entries {
				fmt.Printf("%s@%s\t%s\n", e.Name, e.Version, e.Description)
			}
			return nil
		},
	}
	registryCmd.AddCommand(registryListCmd)

	var registryInstallCmd = &cobra.Command{
		Use:   "install [manifest-path]",
		Short: "Install a command from a .cpl.json manifest file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := registry.OpenDefault()
			if err != nil {
				return fmt.Errorf("failed to open registry: %w", err)
			}
			manifest, err := cpl.LoadFile(args[0])
			if err != nil {
				return fmt.Errorf("failed to load manifest: %w", err)
			}
			if err := reg.Install(manifest); err != nil {
				return fmt.Errorf("failed to install: %w", err)
			}
			fmt.Printf("Installed %s@%s\n", manifest.Name, manifest.Version)
			return nil
		},
	}
	registryCmd.AddCommand(registryInstallCmd)

	var registryRemoveCmd = &cobra.Command{
		Use:   "remove [command-name]",
		Short: "Remove an installed command",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := registry.OpenDefault()
			if err != nil {
				return fmt.Errorf("failed to open registry: %w", err)
			}
			if err := reg.Remove(args[0]); err != nil {
				return fmt.Errorf("failed to remove: %w", err)
			}
			fmt.Printf("Removed %s\n", args[0])
			return nil
		},
	}
	registryCmd.AddCommand(registryRemoveCmd)

	// --- inspect subcommand ---
	var inspectCmd = &cobra.Command{
		Use:   "inspect [command-name]",
		Short: "Show command manifest details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := registry.OpenDefault()
			if err != nil {
				return fmt.Errorf("failed to open registry: %w", err)
			}
			manifest, err := reg.Get(args[0])
			if err != nil {
				return fmt.Errorf("command not found: %w", err)
			}
			fmt.Println(cpl.FormatManifest(manifest))
			return nil
		},
	}
	rootCmd.AddCommand(inspectCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
