// Package validator provides CPL manifest validation, sandbox security testing,
// and pipeline compatibility verification for UniCLI OS.
//
// Usage:
//
//	// Validate a manifest
//	v := validator.NewManifestValidator("")
//	result := v.ValidateFile("examples/image.resize.cpl.json")
//	if !result.Valid {
//	    for _, err := range result.Errors {
//	        fmt.Println(err)
//	    }
//	}
//
//	// Run security tests
//	tester := validator.NewSecurityTester()
//	report := tester.RunAll("alpine:latest")
//
//	// Check pipe compatibility
//	checker := validator.NewPipeCompatibilityChecker()
//	cr := checker.Check(validator.OutputFile, validator.InputString)
package validator

import "fmt"

// ValidateManifest is a convenience function that creates a ManifestValidator
// with the default schema path and validates raw JSON data.
func ValidateManifest(data []byte) *ValidationResult {
	return NewManifestValidator("").Validate(data)
}

// ValidateManifestFile is a convenience function that creates a ManifestValidator
// with the default schema path and validates a manifest file.
func ValidateManifestFile(path string) *ValidationResult {
	return NewManifestValidator("").ValidateFile(path)
}

// RunSecurityTests is a convenience function that creates a SecurityTester
// and runs all sandbox security tests.
func RunSecurityTests(imageRef string) *SecurityReport {
	return NewSecurityTester().RunAll(imageRef)
}

// RunPipelineTests runs all pipeline-related tests (compatibility matrix + chain).
// Returns a PipeTestReport.
func RunPipelineTests() *PipeTestReport {
	tester := NewPipelineTester()
	report := &PipeTestReport{
		Results: make([]*TestResult, 0),
	}

	results := []*TestResult{
		tester.RunCompatibilityMatrixTest(),
		tester.RunPipeChainTest(),
		tester.RunBasicPipeTest(""),
	}

	report.AllPassed = true
	for _, r := range results {
		report.Results = append(report.Results, r)
		if r.Status == TestFailed {
			report.AllPassed = false
		}
	}

	return report
}

// PrintValidationResult prints a human-readable validation result.
func PrintValidationResult(vr *ValidationResult) {
	if vr == nil {
		fmt.Println("No validation result")
		return
	}

	fmt.Printf("Manifest: %s\n", vr.ManifestPath)
	fmt.Printf("Result: %s\n", vr.Summary())

	for _, e := range vr.Errors {
		icon := " "
		switch e.Severity {
		case SeverityError:
			icon = "X"
		case SeverityWarning:
			icon = "!"
		case SeverityInfo:
			icon = "i"
		}
		fmt.Printf("  [%s] %s\n", icon, e.Message)
		if e.Field != "" {
			fmt.Printf("        Field: %s\n", e.Field)
		}
	}
}

// PrintSecurityReport prints a human-readable security test report.
func PrintSecurityReport(report *SecurityReport) {
	if report == nil {
		fmt.Println("No security report")
		return
	}

	fmt.Println("=== Sandbox Security Tests ===")
	for _, r := range report.Results {
		icon := "?"
		switch r.Status {
		case TestPassed:
			icon = "PASS"
		case TestFailed:
			icon = "FAIL"
		case TestSkipped:
			icon = "SKIP"
		case TestError:
			icon = "ERR "
		}
		fmt.Printf("  [%s] %s (%dms)\n", icon, r.Name, r.DurationMs)
		if r.Message != "" {
			fmt.Printf("       %s\n", r.Message)
		}
	}
	if report.AllPassed {
		fmt.Println("Result: ALL PASSED")
	} else {
		fmt.Println("Result: SOME TESTS FAILED")
	}
}

// PrintPipeTestReport prints a human-readable pipeline test report.
func PrintPipeTestReport(report *PipeTestReport) {
	if report == nil {
		fmt.Println("No pipeline test report")
		return
	}

	fmt.Println("=== Pipeline Tests ===")
	for _, r := range report.Results {
		icon := "?"
		switch r.Status {
		case TestPassed:
			icon = "PASS"
		case TestFailed:
			icon = "FAIL"
		case TestSkipped:
			icon = "SKIP"
		case TestError:
			icon = "ERR "
		}
		fmt.Printf("  [%s] %s (%dms)\n", icon, r.Name, r.DurationMs)
		if r.Message != "" {
			fmt.Printf("       %s\n", r.Message)
		}
	}
	if report.AllPassed {
		fmt.Println("Result: ALL PASSED")
	} else {
		fmt.Println("Result: SOME TESTS FAILED")
	}
}
