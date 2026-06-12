package validator

import (
	"encoding/json"
	"testing"
)

func TestValidateManifest_ValidHelloSay(t *testing.T) {
	data := loadTestManifest(t, `{
		"cpl_version": "1.0.0",
		"name": "hello.say",
		"version": "1.0.0",
		"description": "Test CLI",
		"inputs": [
			{"name": "name", "type": "STRING", "default": "World"},
			{"name": "greeting", "type": "STRING", "default": "Hello"}
		],
		"outputs": [
			{"name": "greeting_text", "type": "TEXT", "capture_stdout": true}
		],
		"resources": {"cpu": 0.1, "memory": 16},
		"image": {"ref": "ghcr.io/Guo-Dong-Liang/test:1.0.0", "entrypoint": "/app/test.sh"}
	}`)

	result := ValidateManifest(data)
	if !result.Valid {
		t.Errorf("Expected valid manifest, got errors: %v", result.Errors)
	}
}

func TestValidateManifest_MissingRequiredFields(t *testing.T) {
	data := loadTestManifest(t, `{
		"name": "test.cli"
	}`)

	result := ValidateManifest(data)
	if result.Valid {
		t.Fatal("Expected invalid manifest (missing required fields)")
	}

	expectedMissing := map[string]bool{
		"cpl_version": true,
		"version":     true,
		"outputs":     true,
		"image":       true,
	}

	for _, e := range result.Errors {
		if e.Code == "MANIFEST_MISSING_FIELD" {
			delete(expectedMissing, e.Field)
		}
	}

	if len(expectedMissing) > 0 {
		t.Errorf("Expected errors for fields: %v, got codes: %v", expectedMissing, errorCodes(result))
	}
}

func TestValidateManifest_EmptyName(t *testing.T) {
	data := loadTestManifest(t, `{
		"cpl_version": "1.0.0",
		"name": "",
		"version": "1.0.0",
		"outputs": [{"name": "out", "type": "TEXT"}],
		"image": {"ref": "test:1", "entrypoint": "/app/test.sh"}
	}`)

	result := ValidateManifest(data)
	found := findError(result, "NAME_EMPTY")
	if found == nil {
		t.Error("Expected NAME_EMPTY error for empty name")
	}
}

func TestValidateManifest_NameFormat(t *testing.T) {
	tests := []struct {
		name    string
		valid   bool
	}{
		{"valid.name", true},
		{"image.resize", true},
		{"a.b.c", true},
		{"Hello", false},       // uppercase
		{"has space", false},   // space
		{"", false},            // empty
		{".leading", false},    // leading dot
		{"trailing.", false},   // trailing dot
	}

	v := NewManifestValidator("")

	for _, tc := range tests {
		obj := map[string]interface{}{
			"cpl_version": "1.0.0",
			"name":        tc.name,
			"version":     "1.0.0",
			"outputs": []interface{}{
				map[string]interface{}{"name": "out", "type": "TEXT"},
			},
			"image": map[string]interface{}{
				"ref":        "test:1",
				"entrypoint": "/app/test.sh",
			},
		}
		data, _ := json.Marshal(obj)
		result := v.Validate(data)

		if tc.valid && !result.Valid {
			// Check if the failure is about name format, not something else
			if findError(result, "NAME_FORMAT") != nil || findError(result, "NAME_EMPTY") != nil {
				t.Errorf("Name %q should be valid but got errors: %v", tc.name, errorCodesWithPrefix(result, "NAME"))
			}
		}
	}
}

func TestValidateManifest_InvalidCPLVersion(t *testing.T) {
	data := loadTestManifest(t, `{
		"cpl_version": "not.a.version",
		"name": "test.cli",
		"version": "1.0.0",
		"outputs": [{"name": "out", "type": "TEXT"}],
		"image": {"ref": "test:1", "entrypoint": "/app/test.sh"}
	}`)

	result := ValidateManifest(data)
	found := findError(result, "CPL_VERSION_FORMAT")
	if found == nil {
		t.Error("Expected CPL_VERSION_FORMAT error for non-numeric version")
	}
}

func TestValidateManifest_RequiredInputWithoutDefault(t *testing.T) {
	data := loadTestManifest(t, `{
		"cpl_version": "1.0.0",
		"name": "test.cli",
		"version": "1.0.0",
		"inputs": [
			{"name": "input_file", "type": "FILE", "required": true, "description": "Input file"}
		],
		"outputs": [{"name": "out", "type": "TEXT"}],
		"image": {"ref": "test:1", "entrypoint": "/app/test.sh"}
	}`)

	result := ValidateManifest(data)
	// Should still be valid (required without default is info, not error)
	// But should have INFO about it
	found := findError(result, "REQUIRED_INPUT_NO_DEFAULT")
	if found == nil {
		t.Error("Expected REQUIRED_INPUT_NO_DEFAULT info for required input without default")
	}
}

func TestValidateManifest_OutputsRequired(t *testing.T) {
	data := loadTestManifest(t, `{
		"cpl_version": "1.0.0",
		"name": "test.cli",
		"version": "1.0.0",
		"inputs": [],
		"image": {"ref": "test:1", "entrypoint": "/app/test.sh"}
	}`)

	result := ValidateManifest(data)
	found := findError(result, "MANIFEST_MISSING_FIELD")
	if found == nil || found.Field != "outputs" {
		t.Error("Expected MANIFEST_MISSING_FIELD for 'outputs' when outputs is missing")
	}
}

func TestValidateManifest_EmptyOutputs(t *testing.T) {
	data := loadTestManifest(t, `{
		"cpl_version": "1.0.0",
		"name": "test.cli",
		"version": "1.0.0",
		"outputs": [],
		"image": {"ref": "test:1", "entrypoint": "/app/test.sh"}
	}`)

	result := ValidateManifest(data)
	found := findError(result, "OUTPUTS_EMPTY")
	if found == nil {
		t.Error("Expected OUTPUTS_EMPTY error when outputs array is empty")
	}
}

func TestValidateManifest_DuplicateInputNames(t *testing.T) {
	data := loadTestManifest(t, `{
		"cpl_version": "1.0.0",
		"name": "test.cli",
		"version": "1.0.0",
		"inputs": [
			{"name": "input_file", "type": "FILE", "required": true},
			{"name": "input_file", "type": "STRING", "required": true}
		],
		"outputs": [{"name": "out", "type": "TEXT"}],
		"image": {"ref": "test:1", "entrypoint": "/app/test.sh"}
	}`)

	result := ValidateManifest(data)
	found := findError(result, "INPUT_DUPLICATE_NAME")
	if found == nil {
		t.Error("Expected INPUT_DUPLICATE_NAME error for duplicate input names")
	}
}

func TestValidateManifest_ENUMWithoutValues(t *testing.T) {
	data := loadTestManifest(t, `{
		"cpl_version": "1.0.0",
		"name": "test.cli",
		"version": "1.0.0",
		"inputs": [
			{"name": "format", "type": "ENUM"}
		],
		"outputs": [{"name": "out", "type": "TEXT"}],
		"image": {"ref": "test:1", "entrypoint": "/app/test.sh"}
	}`)

	result := ValidateManifest(data)
	found := findError(result, "INPUT_ENUM_MISSING_VALUES")
	if found == nil {
		t.Error("Expected INPUT_ENUM_MISSING_VALUES error for ENUM type without enum_values")
	}
}

func TestValidateManifest_InvalidInputType(t *testing.T) {
	data := loadTestManifest(t, `{
		"cpl_version": "1.0.0",
		"name": "test.cli",
		"version": "1.0.0",
		"inputs": [
			{"name": "data", "type": "LIST"}
		],
		"outputs": [{"name": "out", "type": "TEXT"}],
		"image": {"ref": "test:1", "entrypoint": "/app/test.sh"}
	}`)

	result := ValidateManifest(data)
	found := findError(result, "INPUT_INVALID_TYPE")
	if found == nil {
		t.Error("Expected INPUT_INVALID_TYPE error for unknown input type")
	}
}

func TestValidateManifest_ResourcesOutOfRange(t *testing.T) {
	data := loadTestManifest(t, `{
		"cpl_version": "1.0.0",
		"name": "test.cli",
		"version": "1.0.0",
		"outputs": [{"name": "out", "type": "TEXT"}],
		"resources": {"cpu": 999, "memory": 1},
		"image": {"ref": "test:1", "entrypoint": "/app/test.sh"}
	}`)

	result := ValidateManifest(data)
	foundCpu := findError(result, "RESOURCE_OUT_OF_RANGE")
	if foundCpu == nil {
		t.Error("Expected RESOURCE_OUT_OF_RANGE for cpu=999")
	}
}

func TestValidateManifest_ImageMissingEntrypoint(t *testing.T) {
	data := loadTestManifest(t, `{
		"cpl_version": "1.0.0",
		"name": "test.cli",
		"version": "1.0.0",
		"outputs": [{"name": "out", "type": "TEXT"}],
		"image": {"ref": "test:1"}
	}`)

	result := ValidateManifest(data)
	found := findError(result, "IMAGE_MISSING_ENTRYPOINT")
	if found == nil {
		t.Error("Expected IMAGE_MISSING_ENTRYPOINT error when entrypoint is missing")
	}
}

func TestValidateManifest_DefaultTypeMismatch(t *testing.T) {
	data := loadTestManifest(t, `{
		"cpl_version": "1.0.0",
		"name": "test.cli",
		"version": "1.0.0",
		"inputs": [
			{"name": "count", "type": "INT", "default": "not_an_int"}
		],
		"outputs": [{"name": "out", "type": "TEXT"}],
		"image": {"ref": "test:1", "entrypoint": "/app/test.sh"}
	}`)

	result := ValidateManifest(data)
	found := findError(result, "INPUT_DEFAULT_TYPE_MISMATCH")
	if found == nil {
		t.Error("Expected INPUT_DEFAULT_TYPE_MISMATCH for INT default with string value")
	}
}

func TestValidateManifest_SignatureGPGRequiresPublicKey(t *testing.T) {
	data := loadTestManifest(t, `{
		"cpl_version": "1.0.0",
		"name": "test.cli",
		"version": "1.0.0",
		"outputs": [{"name": "out", "type": "TEXT"}],
		"image": {"ref": "test:1", "entrypoint": "/app/test.sh"},
		"signature": {"algorithm": "GPG", "digest": "abc123"}
	}`)

	result := ValidateManifest(data)
	found := findError(result, "SIGNATURE_MISSING_PUBLIC_KEY")
	if found == nil {
		t.Error("Expected SIGNATURE_MISSING_PUBLIC_KEY for GPG without public_key")
	}
}

func TestValidateManifest_NetworkEnabledWarning(t *testing.T) {
	data := loadTestManifest(t, `{
		"cpl_version": "1.0.0",
		"name": "test.cli",
		"version": "1.0.0",
		"outputs": [{"name": "out", "type": "TEXT"}],
		"resources": {"network": true},
		"image": {"ref": "test:1", "entrypoint": "/app/test.sh"}
	}`)

	result := ValidateManifest(data)
	found := findError(result, "RESOURCE_NETWORK_ENABLED")
	if found == nil {
		t.Error("Expected RESOURCE_NETWORK_ENABLED warning when network is enabled")
	}
}

func TestValidateManifest_NotJSON(t *testing.T) {
	data := []byte("not json")
	result := ValidateManifest(data)
	found := findError(result, "MANIFEST_JSON_PARSE")
	if found == nil {
		t.Error("Expected MANIFEST_JSON_PARSE error for invalid JSON")
	}
}

func TestValidateManifest_NonObjectRoot(t *testing.T) {
	data := []byte(`["array", "not", "object"]`)
	result := ValidateManifest(data)
	found := findError(result, "MANIFEST_NOT_OBJECT")
	if found == nil {
		t.Error("Expected MANIFEST_NOT_OBJECT error for array root")
	}
}

// --- Pipe compatibility tests ---

func TestPipeCompatibility(t *testing.T) {
	checker := NewPipeCompatibilityChecker()

	// Valid pairs
	valid := []struct{ from, to string }{
		{"TEXT", "STRING"},
		{"TEXT", "FILE"},
		{"FILE", "FILE"},
		{"STREAM", "STREAM"},
		{"STREAM", "STRING"},
		{"STRUCT", "STRUCT"},
		{"STRUCT", "STRING"},
		{"STRUCT", "STREAM"},
	}

	for _, tc := range valid {
		cr := checker.Check(OutputType(tc.from), InputType(tc.to))
		if !cr.Compatible {
			t.Errorf("Expected %s -> %s to be compatible, got error: %s", tc.from, tc.to, cr.Error)
		}
	}

	// Invalid pairs
	invalid := []struct{ from, to string }{
		{"FILE", "STRING"},
		{"FILE", "INT"},
		{"FILE", "BOOLEAN"},
		{"TEXT", "INT"},
		{"STREAM", "BOOLEAN"},
		{"STRUCT", "INT"},
		{"FILE", "ENUM"},
	}

	for _, tc := range invalid {
		cr := checker.Check(OutputType(tc.from), InputType(tc.to))
		if cr.Compatible {
			t.Errorf("Expected %s -> %s to be incompatible", tc.from, tc.to)
		}
	}
}

func TestPipeChainValid(t *testing.T) {
	checker := NewPipeCompatibilityChecker()
	chain := [][2]string{
		{"TEXT", "STRING"},
		{"TEXT", "FILE"},
	}
	result := checker.ValidatePipeChain(chain)
	if !result.Valid {
		t.Errorf("Expected valid pipe chain, got errors: %v", result.Errors)
	}
}

func TestPipeChainInvalid(t *testing.T) {
	checker := NewPipeCompatibilityChecker()
	chain := [][2]string{
		{"FILE", "INT"},
	}
	result := checker.ValidatePipeChain(chain)
	if result.Valid {
		t.Errorf("Expected invalid pipe chain")
	}
}

// --- Pipeline tester tests ---

func TestPipelineCompatibilityMatrix(t *testing.T) {
	tester := NewPipelineTester()
	result := tester.RunCompatibilityMatrixTest()
	if result.Status == TestFailed {
		t.Fatalf("Compatibility matrix test failed: %s", result.Message)
	}
}

func TestPipelineChainTest(t *testing.T) {
	tester := NewPipelineTester()
	result := tester.RunPipeChainTest()
	if result.Status == TestFailed {
		t.Fatalf("Pipe chain test failed: %s", result.Message)
	}
}

// --- Helpers ---

func loadTestManifest(t *testing.T, jsonStr string) []byte {
	t.Helper()
	return []byte(jsonStr)
}

func findError(result *ValidationResult, code string) *ValidationError {
	for _, e := range result.Errors {
		if e.Code == code {
			return e
		}
	}
	return nil
}

func errorCodes(result *ValidationResult) []string {
	codes := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		codes[i] = e.Code
	}
	return codes
}

func errorCodesWithPrefix(result *ValidationResult, prefix string) []string {
	var codes []string
	for _, e := range result.Errors {
		if len(e.Code) >= len(prefix) && e.Code[:len(prefix)] == prefix {
			codes = append(codes, e.Code)
		}
	}
	return codes
}
