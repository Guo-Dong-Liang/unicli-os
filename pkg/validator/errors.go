// Package validator provides CPL manifest validation, sandbox security testing,
// and pipeline compatibility verification for UniCLI OS.
package validator

import "fmt"

// Severity represents the severity level of a validation issue.
type Severity int

const (
	SeverityError   Severity = iota // Must fix — breaks execution
	SeverityWarning                 // Should fix — best practice violation
	SeverityInfo                    // Informational — suggestion
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "ERROR"
	case SeverityWarning:
		return "WARNING"
	case SeverityInfo:
		return "INFO"
	default:
		return "UNKNOWN"
	}
}

// ValidationError represents a single validation issue found in a manifest.
type ValidationError struct {
	Code     string   // Machine-readable error code (e.g. "MANIFEST_MISSING_FIELD")
	Field    string   // JSON path to the problematic field (e.g. "image.ref")
	Message  string   // Human-readable description
	Severity Severity // Error, Warning, or Info
}

func (ve *ValidationError) Error() string {
	return fmt.Sprintf("[%s] %s: %s (field: %s)", ve.Severity, ve.Code, ve.Message, ve.Field)
}

// ValidationResult collects all issues found during manifest validation.
type ValidationResult struct {
	Valid        bool              // True if no errors (warnings/info allowed)
	Errors       []*ValidationError // All issues found
	ManifestPath string            // Source file path (if loaded from file)
}

// NewValidationResult creates a new ValidationResult.
func NewValidationResult() *ValidationResult {
	return &ValidationResult{
		Valid:  true,
		Errors: make([]*ValidationError, 0),
	}
}

// Add appends a validation issue and flips Valid to false if severity is Error.
func (vr *ValidationResult) Add(ve *ValidationError) {
	vr.Errors = append(vr.Errors, ve)
	if ve.Severity == SeverityError {
		vr.Valid = false
	}
}

// ErrorCount returns the number of errors (vs warnings/info).
func (vr *ValidationResult) ErrorCount() int {
	count := 0
	for _, e := range vr.Errors {
		if e.Severity == SeverityError {
			count++
		}
	}
	return count
}

// WarningCount returns the number of warnings.
func (vr *ValidationResult) WarningCount() int {
	count := 0
	for _, e := range vr.Errors {
		if e.Severity == SeverityWarning {
			count++
		}
	}
	return count
}

// Summary returns a human-readable summary of the validation result.
func (vr *ValidationResult) Summary() string {
	if vr.Valid {
		return fmt.Sprintf("PASS (%d warnings, %d info)", vr.WarningCount(), vr.WarningCount())
	}
	return fmt.Sprintf("FAIL (%d errors, %d warnings)", vr.ErrorCount(), vr.WarningCount())
}

// --- Security / Pipeline test result types ---

// TestStatus represents the outcome of a security or pipeline test.
type TestStatus int

const (
	TestPassed  TestStatus = iota
	TestFailed
	TestSkipped // e.g. Docker not available
	TestError   // Internal error running the test
)

func (ts TestStatus) String() string {
	switch ts {
	case TestPassed:
		return "PASS"
	case TestFailed:
		return "FAIL"
	case TestSkipped:
		return "SKIP"
	case TestError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// TestResult represents the outcome of a single security or pipeline test.
type TestResult struct {
	Name        string     // Test name
	Description string     // What this test verifies
	Status      TestStatus // Outcome
	Message     string     // Human-readable detail on failure
	DurationMs  int64      // Execution time in milliseconds
}

// SecurityReport collects all sandbox security test results.
type SecurityReport struct {
	Results    []*TestResult
	AllPassed  bool
}

// PipeTestReport collects pipeline test results.
type PipeTestReport struct {
	Results   []*TestResult
	AllPassed bool
}
