package validator

import "fmt"

// PipeCompatibilityChecker validates type compatibility between pipeline stages
// as defined by the CPL v1.0 pipe compatibility matrix (see cpl-spec-v1.md §5.3).
type PipeCompatibilityChecker struct{}

// NewPipeCompatibilityChecker creates a new checker.
func NewPipeCompatibilityChecker() *PipeCompatibilityChecker {
	return &PipeCompatibilityChecker{}
}

// InputType is the string representation of an input type.
type InputType string

// OutputType is the string representation of an output type.
type OutputType string

const (
	InputFile   InputType = "FILE"
	InputString InputType = "STRING"
	InputInt    InputType = "INT"
	InputFloat  InputType = "FLOAT"
	InputBool   InputType = "BOOLEAN"
	InputEnum   InputType = "ENUM"
	InputStream InputType = "STREAM"

	OutputFile   OutputType = "FILE"
	OutputText   OutputType = "TEXT"
	OutputStream OutputType = "STREAM"
	OutputStruct OutputType = "STRUCT"
)

// CompatibilityResult describes whether two stages are pipe-compatible.
type CompatibilityResult struct {
	Compatible bool   // True if types can be connected
	Warning    string // Non-empty if the conversion is lossy
	Error      string // Non-empty if incompatible
}

// Check checks if a left output can be piped into a right input.
// Returns the compatibility result with any warnings about lossy conversions.
func (pc *PipeCompatibilityChecker) Check(leftType OutputType, rightType InputType) *CompatibilityResult {
	key := string(leftType) + "->" + string(rightType)

	switch key {
	// Exact/trivial matches
	case "TEXT->STRING":
		return &CompatibilityResult{Compatible: true}
	case "TEXT->FILE":
		return &CompatibilityResult{Compatible: true, Warning: "TEXT output will be written to a temp file"}
	case "TEXT->STREAM":
		return &CompatibilityResult{Compatible: true, Warning: "TEXT output will be passed as stream"}
	case "FILE->STRING":
		return &CompatibilityResult{Compatible: false, Error: "FILE output cannot be piped as STRING; requires file content extraction"}
	case "FILE->FILE":
		return &CompatibilityResult{Compatible: true}
	case "FILE->STREAM":
		return &CompatibilityResult{Compatible: false, Error: "FILE output cannot be passed as STREAM input directly"}
	case "STREAM->STREAM":
		return &CompatibilityResult{Compatible: true}
	case "STREAM->STRING":
		return &CompatibilityResult{Compatible: true, Warning: "STREAM output will be buffered as STRING"}
	case "STREAM->FILE":
		return &CompatibilityResult{Compatible: true, Warning: "STREAM output will be written to temp file"}
	case "STRUCT->STRUCT":
		return &CompatibilityResult{Compatible: true}
	case "STRUCT->STRING":
		return &CompatibilityResult{Compatible: true, Warning: "STRUCT output will be serialized as JSON string"}
	case "STRUCT->FILE":
		return &CompatibilityResult{Compatible: true, Warning: "STRUCT output will be serialized to temp file"}
	case "STRUCT->STREAM":
		return &CompatibilityResult{Compatible: true}

	// Incompatible — STREAM/STRUCT cannot feed into INT/FLOAT/BOOLEAN/ENUM
	default:
		// Check if right side is a primitive type that can't consume structured data
		switch rightType {
		case InputInt, InputFloat, InputBool, InputEnum:
			return &CompatibilityResult{
				Compatible: false,
				Error:      fmt.Sprintf("Cannot pipe %s output into %s input; type mismatch", leftType, rightType),
			}
		case InputFile:
			// FILE outputs can feed into FILE inputs
			if leftType == OutputFile {
				return &CompatibilityResult{Compatible: true}
			}
			return &CompatibilityResult{
				Compatible: false,
				Error:      fmt.Sprintf("Cannot pipe %s output into FILE input; expected file path", leftType),
			}
		default:
			return &CompatibilityResult{
				Compatible: false,
				Error:      fmt.Sprintf("Incompatible pipe types: %s → %s", leftType, rightType),
			}
	}
	}
}

// ValidatePipeChain validates an entire pipe chain for type compatibility.
// stages is a list of (output_type, input_type) pairs for each pipe segment.
func (pc *PipeCompatibilityChecker) ValidatePipeChain(stages [][2]string) *ValidationResult {
	result := NewValidationResult()

	for i, pair := range stages {
		left := OutputType(pair[0])
		right := InputType(pair[1])
		cr := pc.Check(left, right)
		if !cr.Compatible {
			result.Add(&ValidationError{
				Code:     "PIPE_TYPE_MISMATCH",
				Field:    fmt.Sprintf("pipe[%d]", i),
				Message:  fmt.Sprintf("Stage %d: %s → %s: %s", i, left, right, cr.Error),
				Severity: SeverityError,
			})
		} else if cr.Warning != "" {
			result.Add(&ValidationError{
				Code:     "PIPE_LOSSY_CONVERSION",
				Field:    fmt.Sprintf("pipe[%d]", i),
				Message:  fmt.Sprintf("Stage %d: %s → %s: %s", i, left, right, cr.Warning),
				Severity: SeverityWarning,
			})
		}
	}

	return result
}
