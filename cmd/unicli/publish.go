package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Guo-Dong-Liang/unicli-os/pkg/cpl"
	"github.com/Guo-Dong-Liang/unicli-os/pkg/publish"
)

// --- Gitea Config ---

func getDefaultGiteaURL() string {
	if url := os.Getenv("UNICLI_GITEA_URL"); url != "" {
		return url
	}
	return "http://localhost:3000"
}

// --- Auth Token ---

func getPublishToken() string {
	cfgPath := getUniclircPath()
	if data, err := os.ReadFile(cfgPath); err == nil {
		var cfg map[string]string
		if json.Unmarshal(data, &cfg) == nil {
			if token, ok := cfg["gitea_token"]; ok {
				return token
			}
		}
	}
	return ""
}

func setPublishToken(token string) {
	cfgPath := getUniclircPath()
	os.MkdirAll(filepath.Dir(cfgPath), 0755)

	// Read existing config
	cfg := map[string]string{"remote_registry": getRemoteURL()}
	if data, err := os.ReadFile(cfgPath); err == nil {
		json.Unmarshal(data, &cfg)
	}
	cfg["gitea_token"] = token
	cfg["gitea_url"] = getDefaultGiteaURL()

	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(cfgPath, data, 0644)
}

// getPublishBackend creates and configures a publish Backend from stored config.
func getPublishBackend() (publish.Backend, error) {
	cfgPath := getUniclircPath()
	token := getPublishToken()
	giteaURL := getDefaultGiteaURL()

	// Read full config from file
	backendType := "gitea"
	owner := "admin"
	repo := "unicli-os"
	branch := "main"
	basePath := "registry/tools"

	if data, err := os.ReadFile(cfgPath); err == nil {
		var cfg map[string]string
		if json.Unmarshal(data, &cfg) == nil {
			if bt, ok := cfg["backend"]; ok {
				backendType = bt
			}
			if o, ok := cfg["owner"]; ok {
				owner = o
			}
			if r, ok := cfg["repo"]; ok {
				repo = r
			}
			if b, ok := cfg["branch"]; ok {
				branch = b
			}
			if bp, ok := cfg["base_path"]; ok {
				basePath = bp
			}
		}
	}

	pubCfg := publish.Config{
		BackendType: backendType,
		BaseURL:     giteaURL,
		Token:       token,
		Owner:       owner,
		Repo:        repo,
		Branch:      branch,
		BasePath:    basePath,
	}

	return publish.New(pubCfg)
}

// --- Publish ---

func publishTool(toolDir string) {
	// Expand path
	toolDir = expandPath(toolDir)

	// Validate tool directory
	info, err := os.Stat(toolDir)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: '%s' is not a valid directory\n", toolDir)
		os.Exit(1)
	}

	// Read manifest
	toolName := filepath.Base(toolDir)
	manifestPath := filepath.Join(toolDir, toolName+".cpl.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: no manifest found: %s\n", manifestPath)
		fmt.Fprintf(os.Stderr, "  Run 'unicli init %s' first to create one\n", toolName)
		os.Exit(1)
	}

	// Validate manifest JSON
	var manifest cpl.CPLManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid manifest: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("📦 Publishing: %s v%s\n", manifest.Name, manifest.Version)
	fmt.Printf("   Description: %s\n", manifest.Description)
	fmt.Printf("   From: %s\n\n", toolDir)

	// Check auth
	token := getPublishToken()
	if token == "" {
		fmt.Println("⚠  No publish token configured.")
		fmt.Println("   Run: unicli registry login gitea <token>")
		fmt.Println("   Get token from your self-hosted Gitea server (Settings > Applications)")
		os.Exit(1)
	}

	// Get publish backend
	backend, err := getPublishBackend()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to configure publish backend: %v\n", err)
		os.Exit(1)
	}
	basePath := "registry/tools"

	// Collect files to upload
	type fileToUpload struct {
		path    string
		content []byte
	}
	var files []fileToUpload

	// Read manifest
	manifestContent, _ := os.ReadFile(manifestPath)
	files = append(files, fileToUpload{
		path:    fmt.Sprintf("%s/%s/%s.cpl.json", basePath, toolName, toolName),
		content: manifestContent,
	})

	// Read entrypoint
	entrypoint := filepath.Base(manifest.Image.Entrypoint)
	entryPath := filepath.Join(toolDir, entrypoint)
	if entryData, err := os.ReadFile(entryPath); err == nil {
		files = append(files, fileToUpload{
			path:    fmt.Sprintf("%s/%s/%s", basePath, toolName, entrypoint),
			content: entryData,
		})
	}

	// Also upload any .sh or .py files in the directory
	entries, _ := os.ReadDir(toolDir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		fn := e.Name()
		if fn == manifest.Name+".cpl.json" || fn == entrypoint {
			continue
		}
		if strings.HasSuffix(fn, ".sh") || strings.HasSuffix(fn, ".py") || strings.HasSuffix(fn, ".json") {
			data, _ := os.ReadFile(filepath.Join(toolDir, fn))
			files = append(files, fileToUpload{
				path:    fmt.Sprintf("%s/%s/%s", basePath, toolName, fn),
				content: data,
			})
		}
	}

	// Upload via backend
	successCount := 0
	for _, f := range files {
		if err := backend.Upload(f.path, f.content); err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠ Failed to upload %s: %v\n", f.path, err)
			continue
		}
		fmt.Printf("  ✅ Uploaded: %s\n", f.path)
		successCount++
	}

	if successCount > 0 {
		fmt.Printf("\n✅ Published '%s' (%d/%d files)\n", toolName, successCount, len(files))
	} else {
		fmt.Fprintf(os.Stderr, "\n❌ Publish failed\n")
		os.Exit(1)
	}
}
