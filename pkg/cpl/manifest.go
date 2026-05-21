package cpl

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Manifest represents a parsed CPL command manifest (.cpl.json)
type Manifest struct {
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	Description string                 `json:"description"`
	Inputs      []Input                `json:"inputs,omitempty"`
	Outputs     []Output               `json:"outputs,omitempty"`
	Resources   *ResourceRequirement   `json:"resources,omitempty"`
	Network     string                 `json:"network,omitempty"`
	ImageRef    string                 `json:"image_ref"`
	Entrypoint  string                 `json:"entrypoint"`
	Workdir     string                 `json:"workdir,omitempty"`
	Env         map[string]string      `json:"env,omitempty"`
	Mounts      []Mount                `json:"mounts,omitempty"`
	Signature   *Signature             `json:"signature,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type Input struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"` // STRING, INT, FLOAT, BOOL, FILE, ENUM, STREAM
	Required    bool     `json:"required,omitempty"`
	Description string   `json:"description,omitempty"`
	Default     string   `json:"default,omitempty"`
	EnumValues  []string `json:"enum_values,omitempty"`
}

type Output struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // TEXT, FILE, STREAM, STRUCT, EVENT
	Description string `json:"description,omitempty"`
}

type ResourceRequirement struct {
	CPUCores float64 `json:"cpu_cores,omitempty"`
	MemoryMB int     `json:"memory_mb,omitempty"`
	DiskMB   int     `json:"disk_mb,omitempty"`
	GPUCount int     `json:"gpu_count,omitempty"`
	GPUType  string  `json:"gpu_type,omitempty"`
}

type Mount struct {
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"read_only,omitempty"`
	Description   string `json:"description,omitempty"`
}

type Signature struct {
	PublicKey string `json:"public_key"`
	Signature string `json:"signature"`
	Algorithm string `json:"algorithm"`
}

// LoadFile parses a CPL manifest from a JSON file
func LoadFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest file: %w", err)
	}
	return Parse(data)
}

// Parse parses a CPL manifest from JSON bytes
func Parse(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	// Set defaults
	if m.Workdir == "" {
		m.Workdir = "/workspace"
	}
	if m.Network == "" {
		m.Network = "none"
	}

	// Validate required fields
	if m.Name == "" {
		return nil, fmt.Errorf("manifest: name is required")
	}
	if m.Version == "" {
		return nil, fmt.Errorf("manifest: version is required")
	}
	if m.ImageRef == "" {
		return nil, fmt.Errorf("manifest: image_ref is required")
	}
	if m.Entrypoint == "" {
		return nil, fmt.Errorf("manifest: entrypoint is required")
	}

	return &m, nil
}

// GetDefaultInput returns the value of a named input, using default if not set
func (m *Manifest) GetDefaultInput(name string) (string, bool) {
	for _, in := range m.Inputs {
		if in.Name == name {
			if in.Default != "" {
				return in.Default, true
			}
			return "", false
		}
	}
	return "", false
}

// FormatManifest returns a human-readable table of the manifest
func FormatManifest(m *Manifest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "┌─────────────┬─────────────────────────┐\n")
	fmt.Fprintf(&b, "│ Name        │ %-23s │\n", m.Name)
	fmt.Fprintf(&b, "│ Version     │ %-23s │\n", m.Version)
	fmt.Fprintf(&b, "│ Description │ %-23s │\n", truncate(m.Description, 23))
	fmt.Fprintf(&b, "│ Image       │ %-23s │\n", truncate(m.ImageRef, 23))
	fmt.Fprintf(&b, "│ Entrypoint  │ %-23s │\n", m.Entrypoint)
	fmt.Fprintf(&b, "│ Network     │ %-23s │\n", m.Network)
	if m.Resources != nil {
		fmt.Fprintf(&b, "│ CPU         │ %-23.1f │\n", m.Resources.CPUCores)
		fmt.Fprintf(&b, "│ Memory      │ %-23d │\n", m.Resources.MemoryMB)
	}
	fmt.Fprintf(&b, "├─────────────┼─────────────────────────┤\n")
	fmt.Fprintf(&b, "│ Inputs      │                         │\n")
	for _, in := range m.Inputs {
		req := ""
		if in.Required {
			req = " (required)"
		}
		fmt.Fprintf(&b, "│  %-10s │ %-23s │\n", in.Name, in.Type+req)
	}
	fmt.Fprintf(&b, "├─────────────┼─────────────────────────┤\n")
	fmt.Fprintf(&b, "│ Outputs     │                         │\n")
	for _, out := range m.Outputs {
		fmt.Fprintf(&b, "│  %-10s │ %-23s │\n", out.Name, out.Type)
	}
	fmt.Fprintf(&b, "└─────────────┴─────────────────────────┘\n")
	return b.String()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
