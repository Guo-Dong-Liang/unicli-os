package validator

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ManifestValidator validates CPL manifest JSON files against the schema and
// performs cross-field semantic checks that the JSON Schema alone cannot express.
type ManifestValidator struct {
	SchemaPath string // Path to the JSON Schema file
}

// NewManifestValidator creates a validator using the default schema path.
// If schemaPath is empty, it defaults to "schemas/cpl-manifest.schema.json".
func NewManifestValidator(schemaPath string) *ManifestValidator {
	if schemaPath == "" {
		schemaPath = "schemas/cpl-manifest.schema.json"
	}
	return &ManifestValidator{SchemaPath: schemaPath}
}

// ValidateFile loads a manifest JSON file and validates it.
func (mv *ManifestValidator) ValidateFile(path string) *ValidationResult {
	result := NewValidationResult()
	result.ManifestPath = path

	data, err := os.ReadFile(path)
	if err != nil {
		result.Add(&ValidationError{
			Code:     "MANIFEST_READ_ERROR",
			Field:    "(file)",
			Message:  fmt.Sprintf("Cannot read manifest file: %v", err),
			Severity: SeverityError,
		})
		return result
	}

	return mv.Validate(data)
}

// Validate checks a raw JSON manifest byte slice.
func (mv *ManifestValidator) Validate(data []byte) *ValidationResult {
	result := NewValidationResult()

	// Step 1: Parse JSON
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		result.Add(&ValidationError{
			Code:     "MANIFEST_JSON_PARSE",
			Field:    "(root)",
			Message:  fmt.Sprintf("Invalid JSON: %v", err),
			Severity: SeverityError,
		})
		return result
	}

	obj, ok := raw.(map[string]interface{})
	if !ok {
		result.Add(&ValidationError{
			Code:     "MANIFEST_NOT_OBJECT",
			Field:    "(root)",
			Message:  "Root value must be a JSON object",
			Severity: SeverityError,
		})
		return result
	}

	// Step 2: Basic structure validation
	mv.validateRequiredFields(result, obj)
	if !result.Valid {
		// Can't do further validation without required fields
		return result
	}

	// Step 3: Field-type specific validation
	mv.validateCPLVersion(result, obj)
	mv.validateName(result, obj)
	mv.validateVersion(result, obj)
	mv.validateInputs(result, obj)
	mv.validateOutputs(result, obj)
	mv.validateResources(result, obj)
	mv.validateImageSpec(result, obj)
	mv.validateSignature(result, obj)

	// Step 4: Cross-field validations
	mv.validateCrossField(result, obj)

	return result
}

// --- Required fields ---

var requiredFields = []string{"cpl_version", "name", "version", "outputs", "image"}

func (mv *ManifestValidator) validateRequiredFields(r *ValidationResult, obj map[string]interface{}) {
	for _, field := range requiredFields {
		if _, ok := obj[field]; !ok {
			r.Add(&ValidationError{
				Code:     "MANIFEST_MISSING_FIELD",
				Field:    field,
				Message:  fmt.Sprintf("Required field '%s' is missing", field),
				Severity: SeverityError,
			})
		}
	}
}

// --- cpl_version ---

func (mv *ManifestValidator) validateCPLVersion(r *ValidationResult, obj map[string]interface{}) {
	val, ok := obj["cpl_version"].(string)
	if !ok {
		r.Add(&ValidationError{
			Code:     "CPL_VERSION_TYPE",
			Field:    "cpl_version",
			Message:  "cpl_version must be a string (e.g. \"1.0.0\")",
			Severity: SeverityError,
		})
		return
	}

	// Pattern: MAJOR.MINOR.PATCH
	parts := strings.Split(val, ".")
	if len(parts) != 3 {
		r.Add(&ValidationError{
			Code:     "CPL_VERSION_FORMAT",
			Field:    "cpl_version",
			Message:  fmt.Sprintf("cpl_version must be MAJOR.MINOR.PATCH format, got %q", val),
			Severity: SeverityError,
		})
		return
	}

	for _, p := range parts {
		if len(p) == 0 {
			r.Add(&ValidationError{
				Code:     "CPL_VERSION_FORMAT",
				Field:    "cpl_version",
				Message:  fmt.Sprintf("Invalid version segment in %q", val),
				Severity: SeverityError,
			})
			return
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				r.Add(&ValidationError{
					Code:     "CPL_VERSION_FORMAT",
					Field:    "cpl_version",
					Message:  fmt.Sprintf("Version segments must be numeric, got %q", val),
					Severity: SeverityError,
				})
				return
			}
		}
	}
}

// --- name ---

func (mv *ManifestValidator) validateName(r *ValidationResult, obj map[string]interface{}) {
	val, ok := obj["name"].(string)
	if !ok {
		r.Add(&ValidationError{
			Code:     "NAME_TYPE",
			Field:    "name",
			Message:  "name must be a string",
			Severity: SeverityError,
		})
		return
	}

	if len(val) == 0 {
		r.Add(&ValidationError{
			Code:     "NAME_EMPTY",
			Field:    "name",
			Message:  "name must not be empty",
			Severity: SeverityError,
		})
		return
	}

	if len(val) > 128 {
		r.Add(&ValidationError{
			Code:     "NAME_TOO_LONG",
			Field:    "name",
			Message:  "name must be at most 128 characters",
			Severity: SeverityError,
		})
		return
	}

	// Pattern: ^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*$
	parts := strings.Split(val, ".")
	for _, p := range parts {
		if len(p) == 0 {
			r.Add(&ValidationError{
				Code:     "NAME_FORMAT",
				Field:    "name",
				Message:  fmt.Sprintf("name %q contains empty segment (e.g. double dot)", val),
				Severity: SeverityError,
			})
			return
		}
		for i, c := range p {
			if i == 0 && (c < 'a' || c > 'z') {
				r.Add(&ValidationError{
					Code:     "NAME_FORMAT",
					Field:    "name",
					Message:  fmt.Sprintf("name %q: each segment must start with lowercase letter", val),
					Severity: SeverityError,
				})
				return
			}
			if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
				r.Add(&ValidationError{
					Code:     "NAME_FORMAT",
					Field:    "name",
					Message:  fmt.Sprintf("name %q: segments must be lowercase alphanumeric", val),
					Severity: SeverityError,
				})
				return
			}
		}
	}
}

// --- version ---

func (mv *ManifestValidator) validateVersion(r *ValidationResult, obj map[string]interface{}) {
	val, ok := obj["version"].(string)
	if !ok {
		r.Add(&ValidationError{
			Code:     "VERSION_TYPE",
			Field:    "version",
			Message:  "version must be a string (semver, e.g. \"1.0.0\")",
			Severity: SeverityError,
		})
		return
	}

	// Basic semver check: ^\d+\.\d+\.\d+(-[a-zA-Z0-9]+)?$
	parts := strings.Split(val, ".")
	if len(parts) < 3 {
		r.Add(&ValidationError{
			Code:     "VERSION_FORMAT",
			Field:    "version",
			Message:  fmt.Sprintf("version must be semver format (MAJOR.MINOR.PATCH), got %q", val),
			Severity: SeverityError,
		})
		return
	}

	// Check first three parts are numeric
	for i := 0; i < 3; i++ {
		if len(parts[i]) == 0 {
			r.Add(&ValidationError{
				Code:     "VERSION_FORMAT",
				Field:    "version",
				Message:  fmt.Sprintf("Empty version segment in %q", val),
				Severity: SeverityError,
			})
			return
		}
		for _, c := range parts[i] {
			if c < '0' || c > '9' {
				r.Add(&ValidationError{
					Code:     "VERSION_FORMAT",
					Field:    "version",
					Message:  fmt.Sprintf("Version segments must be numeric, got %q", val),
					Severity: SeverityError,
				})
				return
			}
		}
	}
}

// --- inputs ---

func (mv *ManifestValidator) validateInputs(r *ValidationResult, obj map[string]interface{}) {
	rawInputs, ok := obj["inputs"]
	if !ok {
		// inputs is not required by the schema (minItems: 0)
		return
	}

	inputList, ok := rawInputs.([]interface{})
	if !ok {
		r.Add(&ValidationError{
			Code:     "INPUTS_TYPE",
			Field:    "inputs",
			Message:  "inputs must be an array",
			Severity: SeverityError,
		})
		return
	}

	names := make(map[string]bool)
	for i, raw := range inputList {
		input, ok := raw.(map[string]interface{})
		if !ok {
			r.Add(&ValidationError{
				Code:     "INPUT_ITEM_TYPE",
				Field:    fmt.Sprintf("inputs[%d]", i),
				Message:  "Each input must be a JSON object",
				Severity: SeverityError,
			})
			continue
		}

		mv.validateSingleInput(r, input, i, names)
	}
}

func (mv *ManifestValidator) validateSingleInput(r *ValidationResult, input map[string]interface{}, idx int, names map[string]bool) {
	prefix := fmt.Sprintf("inputs[%d]", idx)

	// name is required
	name, hasName := input["name"].(string)
	if !hasName || name == "" {
		r.Add(&ValidationError{
			Code:     "INPUT_MISSING_NAME",
			Field:    prefix + ".name",
			Message:  fmt.Sprintf("Input at index %d is missing 'name'", idx),
			Severity: SeverityError,
		})
	} else {
		// Check uniqueness
		if names[name] {
			r.Add(&ValidationError{
				Code:     "INPUT_DUPLICATE_NAME",
				Field:    prefix + ".name",
				Message:  fmt.Sprintf("Duplicate input name %q", name),
				Severity: SeverityError,
			})
		}
		names[name] = true
	}

	// type is required and must be one of the valid values
	validInputTypes := map[string]bool{
		"FILE": true, "STRING": true, "INT": true, "FLOAT": true,
		"BOOLEAN": true, "ENUM": true, "STREAM": true,
	}
	inputType, hasType := input["type"].(string)
	if !hasType {
		r.Add(&ValidationError{
			Code:     "INPUT_MISSING_TYPE",
			Field:    prefix + ".type",
			Message:  fmt.Sprintf("Input %q is missing 'type'", name),
			Severity: SeverityError,
		})
	} else if !validInputTypes[inputType] {
		r.Add(&ValidationError{
			Code:     "INPUT_INVALID_TYPE",
			Field:    prefix + ".type",
			Message:  fmt.Sprintf("Input %q has invalid type %q; must be one of FILE, STRING, INT, FLOAT, BOOLEAN, ENUM, STREAM", name, inputType),
			Severity: SeverityError,
		})
	}

	// ENUM type requires enum_values
	if inputType == "ENUM" {
		enumVals, ok := input["enum_values"].([]interface{})
		if !ok || len(enumVals) == 0 {
			r.Add(&ValidationError{
				Code:     "INPUT_ENUM_MISSING_VALUES",
				Field:    prefix + ".enum_values",
				Message:  fmt.Sprintf("Input %q has type ENUM but enum_values is missing or empty", name),
				Severity: SeverityError,
			})
		}
	}

	// Check default value type consistency
	if def, hasDefault := input["default"]; hasDefault {
		mv.validateDefaultType(r, prefix, name, inputType, def)
	}

	// Check position >= 0
	if pos, ok := input["position"]; ok {
		switch v := pos.(type) {
		case float64:
			if v < 0 {
				r.Add(&ValidationError{
					Code:     "INPUT_NEGATIVE_POSITION",
					Field:    prefix + ".position",
					Message:  fmt.Sprintf("Input %q position must be >= 0", name),
					Severity: SeverityError,
				})
			}
		}
	}

	// Check flag format
	if flag, ok := input["flag"].(string); ok && flag != "" {
		if !strings.HasPrefix(flag, "--") {
			r.Add(&ValidationError{
				Code:     "INPUT_FLAG_FORMAT",
				Field:    prefix + ".flag",
				Message:  fmt.Sprintf("Input %q flag %q should start with '--'", name, flag),
				Severity: SeverityWarning,
			})
		}
	}
}

func (mv *ManifestValidator) validateDefaultType(r *ValidationResult, prefix, name, inputType string, def interface{}) {
	field := prefix + ".default"

	switch inputType {
	case "STRING", "FILE", "ENUM":
		if _, ok := def.(string); !ok {
			r.Add(&ValidationError{
				Code:     "INPUT_DEFAULT_TYPE_MISMATCH",
				Field:    field,
				Message:  fmt.Sprintf("Input %q has type %s but default is not a string", name, inputType),
				Severity: SeverityError,
			})
		}
	case "INT":
		switch def.(type) {
		case float64:
			// JSON numbers unmarshal as float64
			if def.(float64) != float64(int64(def.(float64))) {
				r.Add(&ValidationError{
					Code:     "INPUT_DEFAULT_TYPE_MISMATCH",
					Field:    field,
					Message:  fmt.Sprintf("Input %q has type INT but default %v is not an integer", name, def),
					Severity: SeverityError,
				})
			}
		default:
			r.Add(&ValidationError{
				Code:     "INPUT_DEFAULT_TYPE_MISMATCH",
				Field:    field,
				Message:  fmt.Sprintf("Input %q has type INT but default is not a number", name),
				Severity: SeverityError,
			})
		}
	case "FLOAT":
		switch def.(type) {
		case float64:
			// OK
		default:
			r.Add(&ValidationError{
				Code:     "INPUT_DEFAULT_TYPE_MISMATCH",
				Field:    field,
				Message:  fmt.Sprintf("Input %q has type FLOAT but default is not a number", name),
				Severity: SeverityError,
			})
		}
	case "BOOLEAN":
		if _, ok := def.(bool); !ok {
			r.Add(&ValidationError{
				Code:     "INPUT_DEFAULT_TYPE_MISMATCH",
				Field:    field,
				Message:  fmt.Sprintf("Input %q has type BOOLEAN but default is not a boolean", name),
				Severity: SeverityError,
			})
		}
	}
}

// --- outputs ---

func (mv *ManifestValidator) validateOutputs(r *ValidationResult, obj map[string]interface{}) {
	rawOutputs, ok := obj["outputs"]
	if !ok {
		r.Add(&ValidationError{
			Code:     "OUTPUTS_MISSING",
			Field:    "outputs",
			Message:  "outputs is required (at least 1 output)",
			Severity: SeverityError,
		})
		return
	}

	outputList, ok := rawOutputs.([]interface{})
	if !ok {
		r.Add(&ValidationError{
			Code:     "OUTPUTS_TYPE",
			Field:    "outputs",
			Message:  "outputs must be an array",
			Severity: SeverityError,
		})
		return
	}

	if len(outputList) == 0 {
		r.Add(&ValidationError{
			Code:     "OUTPUTS_EMPTY",
			Field:    "outputs",
			Message:  "outputs array must have at least 1 item",
			Severity: SeverityError,
		})
		return
	}

	names := make(map[string]bool)
	for i, raw := range outputList {
		output, ok := raw.(map[string]interface{})
		if !ok {
			r.Add(&ValidationError{
				Code:     "OUTPUT_ITEM_TYPE",
				Field:    fmt.Sprintf("outputs[%d]", i),
				Message:  "Each output must be a JSON object",
				Severity: SeverityError,
			})
			continue
		}

		prefix := fmt.Sprintf("outputs[%d]", i)

		// name required
		name, hasName := output["name"].(string)
		if !hasName || name == "" {
			r.Add(&ValidationError{
				Code:     "OUTPUT_MISSING_NAME",
				Field:    prefix + ".name",
				Message:  fmt.Sprintf("Output at index %d is missing 'name'", i),
				Severity: SeverityError,
			})
		} else {
			if names[name] {
				r.Add(&ValidationError{
					Code:     "OUTPUT_DUPLICATE_NAME",
					Field:    prefix + ".name",
					Message:  fmt.Sprintf("Duplicate output name %q", name),
					Severity: SeverityError,
				})
			}
			names[name] = true
		}

		// type required
		validOutputTypes := map[string]bool{
			"FILE": true, "TEXT": true, "STREAM": true, "STRUCT": true,
		}
		outputType, hasType := output["type"].(string)
		if !hasType {
			r.Add(&ValidationError{
				Code:     "OUTPUT_MISSING_TYPE",
				Field:    prefix + ".type",
				Message:  fmt.Sprintf("Output %q is missing 'type'", name),
				Severity: SeverityError,
			})
		} else if !validOutputTypes[outputType] {
			r.Add(&ValidationError{
				Code:     "OUTPUT_INVALID_TYPE",
				Field:    prefix + ".type",
				Message:  fmt.Sprintf("Output %q has invalid type %q; must be one of FILE, TEXT, STREAM, STRUCT", name, outputType),
				Severity: SeverityError,
			})
		}
	}
}

// --- resources ---

func (mv *ManifestValidator) validateResources(r *ValidationResult, obj map[string]interface{}) {
	raw, ok := obj["resources"]
	if !ok {
		return // resources is optional
	}

	res, ok := raw.(map[string]interface{})
	if !ok {
		r.Add(&ValidationError{
			Code:     "RESOURCES_TYPE",
			Field:    "resources",
			Message:  "resources must be a JSON object",
			Severity: SeverityError,
		})
		return
	}

	// Validate ranges
	ranges := map[string]struct {
		min, max float64
	}{
		"cpu":    {0.1, 64},
		"memory": {16, 65536},
		"disk":   {16, 65536},
		"timeout": {1, 3600},
	}

	for field, rng := range ranges {
		if val, ok := res[field]; ok {
			v, ok := val.(float64)
			if !ok {
				r.Add(&ValidationError{
					Code:     "RESOURCE_TYPE",
					Field:    "resources." + field,
					Message:  fmt.Sprintf("resources.%s must be a number", field),
					Severity: SeverityError,
				})
				continue
			}
			if v < rng.min || v > rng.max {
				r.Add(&ValidationError{
					Code:     "RESOURCE_OUT_OF_RANGE",
					Field:    "resources." + field,
					Message:  fmt.Sprintf("resources.%s = %v is out of range [%.1f, %.0f]", field, v, rng.min, rng.max),
					Severity: SeverityError,
				})
			}
		}
	}

	// Validate booleans
	for _, field := range []string{"network", "gpu"} {
		if val, ok := res[field]; ok {
			if _, ok := val.(bool); !ok {
				r.Add(&ValidationError{
					Code:     "RESOURCE_TYPE",
					Field:    "resources." + field,
					Message:  fmt.Sprintf("resources.%s must be a boolean", field),
					Severity: SeverityError,
				})
			}
		}
	}

	// Warn if network=true
	if net, ok := res["network"].(bool); ok && net {
		r.Add(&ValidationError{
			Code:     "RESOURCE_NETWORK_ENABLED",
			Field:    "resources.network",
			Message:  "Network is enabled — this reduces sandbox isolation",
			Severity: SeverityWarning,
		})
	}
}

// --- image ---

func (mv *ManifestValidator) validateImageSpec(r *ValidationResult, obj map[string]interface{}) {
	raw, ok := obj["image"]
	if !ok {
		return // handled in required fields
	}

	img, ok := raw.(map[string]interface{})
	if !ok {
		r.Add(&ValidationError{
			Code:     "IMAGE_TYPE",
			Field:    "image",
			Message:  "image must be a JSON object with 'ref' and 'entrypoint'",
			Severity: SeverityError,
		})
		return
	}

	// ref required
	ref, hasRef := img["ref"].(string)
	if !hasRef || ref == "" {
		r.Add(&ValidationError{
			Code:     "IMAGE_MISSING_REF",
			Field:    "image.ref",
			Message:  "image.ref is required (e.g. \"ghcr.io/Guo-Dong-Liang/image.resize:1.0.0\")",
			Severity: SeverityError,
		})
	} else {
		// Check ref has tag
		if !strings.Contains(ref, ":") {
			r.Add(&ValidationError{
				Code:     "IMAGE_REF_MISSING_TAG",
				Field:    "image.ref",
				Message:  fmt.Sprintf("image.ref %q should include a tag (e.g. :latest)", ref),
				Severity: SeverityWarning,
			})
		}
	}

	// entrypoint required
	entrypoint, hasEP := img["entrypoint"].(string)
	if !hasEP || entrypoint == "" {
		r.Add(&ValidationError{
			Code:     "IMAGE_MISSING_ENTRYPOINT",
			Field:    "image.entrypoint",
			Message:  "image.entrypoint is required (absolute path inside container)",
			Severity: SeverityError,
		})
	} else if !strings.HasPrefix(entrypoint, "/") {
		r.Add(&ValidationError{
			Code:     "IMAGE_ENTRYPOINT_NOT_ABSOLUTE",
			Field:    "image.entrypoint",
			Message:  fmt.Sprintf("image.entrypoint %q should be an absolute path", entrypoint),
			Severity: SeverityWarning,
		})
	}

	// workdir is optional, default /workspace
	if wd, ok := img["workdir"].(string); ok && wd != "" && !strings.HasPrefix(wd, "/") {
		r.Add(&ValidationError{
			Code:     "IMAGE_WORKDIR_NOT_ABSOLUTE",
			Field:    "image.workdir",
			Message:  fmt.Sprintf("image.workdir %q should be an absolute path", wd),
			Severity: SeverityWarning,
		})
	}

	// user warning if running as root
	if user, ok := img["user"].(string); ok && (user == "root" || user == "0:0") {
		r.Add(&ValidationError{
			Code:     "IMAGE_ROOT_USER",
			Field:    "image.user",
			Message:  "Running as root in container is a security risk; use nobody:nogroup instead",
			Severity: SeverityWarning,
		})
	}

	// env must be string->string
	if env, ok := img["env"].(map[string]interface{}); ok {
		for k, v := range env {
			if _, ok := v.(string); !ok {
				r.Add(&ValidationError{
					Code:     "IMAGE_ENV_VALUE_TYPE",
					Field:    fmt.Sprintf("image.env.%s", k),
					Message:  fmt.Sprintf("Environment variable %s value must be a string", k),
					Severity: SeverityError,
				})
			}
		}
	}
}

// --- signature ---

func (mv *ManifestValidator) validateSignature(r *ValidationResult, obj map[string]interface{}) {
	raw, ok := obj["signature"]
	if !ok {
		return // optional
	}

	sig, ok := raw.(map[string]interface{})
	if !ok {
		r.Add(&ValidationError{
			Code:     "SIGNATURE_TYPE",
			Field:    "signature",
			Message:  "signature must be a JSON object",
			Severity: SeverityError,
		})
		return
	}

	validAlgos := map[string]bool{"SHA256": true, "SHA512": true, "GPG": true}
	algo, ok := sig["algorithm"].(string)
	if !ok {
		r.Add(&ValidationError{
			Code:     "SIGNATURE_MISSING_ALGORITHM",
			Field:    "signature.algorithm",
			Message:  "signature.algorithm is required (SHA256, SHA512, or GPG)",
			Severity: SeverityError,
		})
	} else if !validAlgos[algo] {
		r.Add(&ValidationError{
			Code:     "SIGNATURE_INVALID_ALGORITHM",
			Field:    "signature.algorithm",
			Message:  fmt.Sprintf("Invalid signature algorithm %q; must be SHA256, SHA512, or GPG", algo),
			Severity: SeverityError,
		})
	}

	// digest required
	digest, ok := sig["digest"].(string)
	if !ok || digest == "" {
		r.Add(&ValidationError{
			Code:     "SIGNATURE_MISSING_DIGEST",
			Field:    "signature.digest",
			Message:  "signature.digest is required",
			Severity: SeverityError,
		})
	}

	// GPG requires public_key
	if algo == "GPG" {
		if _, ok := sig["public_key"].(string); !ok || sig["public_key"].(string) == "" {
			r.Add(&ValidationError{
				Code:     "SIGNATURE_MISSING_PUBLIC_KEY",
				Field:    "signature.public_key",
				Message:  "Algorithm is GPG but public_key is missing",
				Severity: SeverityError,
			})
		}
	}
}

// --- Cross-field validation ---

func (mv *ManifestValidator) validateCrossField(r *ValidationResult, obj map[string]interface{}) {
	// Check: required inputs that have no default should be warned about
	// (informational — the runner will catch this at execution time)
	if rawInputs, ok := obj["inputs"].([]interface{}); ok {
		requiredWithoutDefault := make([]string, 0)
		for _, raw := range rawInputs {
			if input, ok := raw.(map[string]interface{}); ok {
				isRequired, _ := input["required"].(bool)
				_, hasDefault := input["default"]
				name, _ := input["name"].(string)
				if isRequired && !hasDefault && name != "" {
					requiredWithoutDefault = append(requiredWithoutDefault, name)
				}
			}
		}
		if len(requiredWithoutDefault) > 0 {
			r.Add(&ValidationError{
				Code:     "REQUIRED_INPUT_NO_DEFAULT",
				Field:    "inputs",
				Message:  fmt.Sprintf("Required inputs without defaults: [%s]; inputs must be provided at runtime", strings.Join(requiredWithoutDefault, ", ")),
				Severity: SeverityInfo,
			})
		}
	}

	// Check: if resources.timeout is too short for the declared resources
	if rawRes, ok := obj["resources"].(map[string]interface{}); ok {
		// No automated cross-check, but could warn if timeout < 10s
		if timeout, ok := rawRes["timeout"].(float64); ok && timeout < 10 {
			r.Add(&ValidationError{
				Code:     "TIMEOUT_VERY_SHORT",
				Field:    "resources.timeout",
				Message:  fmt.Sprintf("Timeout of %.0fs is very short — execution may be interrupted", timeout),
				Severity: SeverityInfo,
			})
		}
	}

	// Check: at least one output with capture_stdout or an output type compatible with piping
	if rawOutputs, ok := obj["outputs"].([]interface{}); ok {
		hasPipeable := false
		for _, raw := range rawOutputs {
			if output, ok := raw.(map[string]interface{}); ok {
				outputType, _ := output["type"].(string)
				captureStdout, _ := output["capture_stdout"].(bool)
				if captureStdout || outputType == "TEXT" || outputType == "STREAM" || outputType == "STRUCT" {
					hasPipeable = true
					break
				}
			}
		}
		if !hasPipeable {
			r.Add(&ValidationError{
				Code:     "NO_PIPEABLE_OUTPUT",
				Field:    "outputs",
				Message:  "No output can be piped (all outputs are FILE type without capture_stdout); this CLI cannot be used in pipelines",
				Severity: SeverityInfo,
			})
		}
	}
}
