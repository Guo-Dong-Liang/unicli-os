package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/admin/unicli-os/pkg/cpl"
)

// Registry manages installed CPL command manifests
type Registry struct {
	baseDir string
}

// Entry is a lightweight summary of an installed command
type Entry struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

// OpenDefault opens the default user registry
func OpenDefault() (*Registry, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}
	baseDir := filepath.Join(home, ".unicli", "registry")
	return Open(baseDir)
}

// Open opens a registry at the given directory
func Open(baseDir string) (*Registry, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("create registry dir: %w", err)
	}
	return &Registry{baseDir: baseDir}, nil
}

// Install adds a command manifest to the registry
func (r *Registry) Install(m *cpl.Manifest) error {
	if m.Name == "" {
		return fmt.Errorf("manifest name is required")
	}

	// Validate the manifest structure
	if err := validateManifest(m); err != nil {
		return fmt.Errorf("invalid manifest: %w", err)
	}

	filename := fmt.Sprintf("%s@%s.cpl.json", m.Name, m.Version)
	path := filepath.Join(r.baseDir, filename)

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	// Create a symlink for "latest" (strip version for easy access)
	latestPath := filepath.Join(r.baseDir, m.Name+".cpl.json")
	_ = os.Remove(latestPath)
	_ = os.Symlink(filename, latestPath)

	return nil
}

// Get retrieves a command manifest by name
func (r *Registry) Get(name string) (*cpl.Manifest, error) {
	// Try exact name first (with .cpl.json)
	path := filepath.Join(r.baseDir, name)
	if !strings.HasSuffix(path, ".cpl.json") {
		path += ".cpl.json"
	}

	// If not found, try without version
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Look for any file starting with name@
		entries, err := os.ReadDir(r.baseDir)
		if err != nil {
			return nil, fmt.Errorf("read registry: %w", err)
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), name+"@") && strings.HasSuffix(e.Name(), ".cpl.json") {
				path = filepath.Join(r.baseDir, e.Name())
				break
			}
		}
		// Try name.cpl.json (latest symlink)
		altPath := filepath.Join(r.baseDir, name+".cpl.json")
		if _, err := os.Stat(altPath); err == nil {
			path = altPath
		}
	}

	return cpl.LoadFile(path)
}

// List returns all installed commands
func (r *Registry) List() ([]Entry, error) {
	entries, err := os.ReadDir(r.baseDir)
	if err != nil {
		return nil, fmt.Errorf("read registry: %w", err)
	}

	var result []Entry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".cpl.json") {
			continue
		}
		// Skip symlinked latest entries (avoid duplicates)
		if e.Type()&os.ModeSymlink != 0 {
			continue
		}

		path := filepath.Join(r.baseDir, e.Name())
		m, err := cpl.LoadFile(path)
		if err != nil {
			continue // skip invalid manifests
		}
		result = append(result, Entry{
			Name:        m.Name,
			Version:     m.Version,
			Description: m.Description,
		})
	}
	return result, nil
}

// Remove uninstalls a command by name
func (r *Registry) Remove(name string) error {
	// Remove latest symlink
	latestPath := filepath.Join(r.baseDir, name+".cpl.json")
	os.Remove(latestPath)

	// Remove versioned files
	entries, err := os.ReadDir(r.baseDir)
	if err != nil {
		return fmt.Errorf("read registry: %w", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), name+"@") && strings.HasSuffix(e.Name(), ".cpl.json") {
			path := filepath.Join(r.baseDir, e.Name())
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("remove %s: %w", e.Name(), err)
			}
		}
	}
	return nil
}

// validateManifest checks a manifest for structural validity
func validateManifest(m *cpl.Manifest) error {
	if m.ImageRef == "" {
		return fmt.Errorf("image_ref is required")
	}
	if m.Entrypoint == "" {
		return fmt.Errorf("entrypoint is required")
	}

	// Validate input names are unique
	seen := map[string]bool{}
	for _, in := range m.Inputs {
		if seen[in.Name] {
			return fmt.Errorf("duplicate input name: %s", in.Name)
		}
		seen[in.Name] = true
	}

	// Validate output names are unique
	seen = map[string]bool{}
	for _, out := range m.Outputs {
		if seen[out.Name] {
			return fmt.Errorf("duplicate output name: %s", out.Name)
		}
		seen[out.Name] = true
	}

	return nil
}
