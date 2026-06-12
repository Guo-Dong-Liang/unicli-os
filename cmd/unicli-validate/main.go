// unicli-validate — CPL manifest validation and sandbox security testing tool.
//
// Usage:
//
//	unicli-validate manifest --file <path>         Validate a CPL manifest
//	unicli-validate security --image <ref>          Run sandbox security tests
//	unicli-validate pipeline                         Run pipeline compatibility tests
//	unicli-validate all --file <path> --image <ref>  Run all tests
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Guo-Dong-Liang/unicli-os/pkg/validator"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	subcommand := os.Args[1]

	switch subcommand {
	case "manifest":
		runManifest()
	case "security":
		runSecurity()
	case "pipeline":
		runPipeline()
	case "all":
		runAll()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`UniCLI OS — Validation Tool

Usage:
  unicli-validate manifest --file <path>         Validate a CPL manifest file
  unicli-validate security [--image <ref>]        Run sandbox security tests
  unicli-validate pipeline                        Run pipeline compatibility tests
  unicli-validate all --file <path> [--image <ref>] Run all tests
  unicli-validate help                            Show this help

Examples:
  unicli-validate manifest --file examples/image.resize.cpl.json
  unicli-validate security --image alpine:latest
  unicli-validate all --file examples/hello.say.cpl.json
`)
}

func runManifest() {
	fs := flag.NewFlagSet("manifest", flag.ExitOnError)
	filePath := fs.String("file", "", "Path to CPL manifest JSON file")
	fs.Parse(os.Args[2:])

	if *filePath == "" {
		fmt.Fprintln(os.Stderr, "Error: --file is required")
		os.Exit(1)
	}

	result := validator.ValidateManifestFile(*filePath)
	validator.PrintValidationResult(result)

	if !result.Valid {
		os.Exit(1)
	}
}

func runSecurity() {
	fs := flag.NewFlagSet("security", flag.ExitOnError)
	imageRef := fs.String("image", "alpine:latest", "Container image to test")
	fs.Parse(os.Args[2:])

	report := validator.RunSecurityTests(*imageRef)
	validator.PrintSecurityReport(report)

	if !report.AllPassed {
		hasFailures := false
		for _, r := range report.Results {
			if r.Status == validator.TestFailed {
				hasFailures = true
				break
			}
		}
		if hasFailures {
			os.Exit(1)
		}
	}
}

func runPipeline() {
	report := validator.RunPipelineTests()
	validator.PrintPipeTestReport(report)

	if !report.AllPassed {
		os.Exit(1)
	}
}

func runAll() {
	fs := flag.NewFlagSet("all", flag.ExitOnError)
	filePath := fs.String("file", "", "Path to CPL manifest JSON file")
	imageRef := fs.String("image", "alpine:latest", "Container image for security tests")
	fs.Parse(os.Args[2:])

	exitCode := 0

	if *filePath != "" {
		fmt.Println(">>> Manifest Validation <<<")
		result := validator.ValidateManifestFile(*filePath)
		validator.PrintValidationResult(result)
		if !result.Valid {
			exitCode = 1
		}
		fmt.Println()
	}

	fmt.Println(">>> Pipeline Tests <<<")
	pipelineReport := validator.RunPipelineTests()
	validator.PrintPipeTestReport(pipelineReport)
	if !pipelineReport.AllPassed {
		exitCode = 1
	}
	fmt.Println()

	fmt.Println(">>> Security Tests <<<")
	securityReport := validator.RunSecurityTests(*imageRef)
	validator.PrintSecurityReport(securityReport)
	if !securityReport.AllPassed {
		for _, r := range securityReport.Results {
			if r.Status == validator.TestFailed {
				exitCode = 1
				break
			}
		}
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}
