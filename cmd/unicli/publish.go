package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/unixcli/unicli-os/pkg/cpl"
)

// --- Gitea Config ---

func getDefaultGiteaURL() string {
	if url := os.Getenv("UNICLI_GITEA_URL"); url != "" {
		return url
	}
	return "http://localhost:3000"
}

// --- Gitea Token ---

func getGiteaToken() string {
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

func setGiteaToken(token string) {
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
	token := getGiteaToken()
	if token == "" {
		fmt.Println("⚠  No Gitea token configured.")
		fmt.Println("   Run: unicli registry login gitea <token>")
		fmt.Println("   Get token from your self-hosted Gitea server (Settings > Applications)")
		os.Exit(1)
	}

	giteaURL := getDefaultGiteaURL()
	owner := "admin"
	repo := "unicli-os"
	branch := "main"
	basePath := "registry/tools"

	// Collect files to upload
	type fileToUpload struct {
		path    string
		content string
	}
	var files []fileToUpload

	// Read manifest
	manifestContent, _ := os.ReadFile(manifestPath)
	files = append(files, fileToUpload{
		path:    fmt.Sprintf("%s/%s/%s.cpl.json", basePath, toolName, toolName),
		content: string(manifestContent),
	})

	// Read entrypoint
	entrypoint := filepath.Base(manifest.Image.Entrypoint)
	entryPath := filepath.Join(toolDir, entrypoint)
	if entryData, err := os.ReadFile(entryPath); err == nil {
		files = append(files, fileToUpload{
			path:    fmt.Sprintf("%s/%s/%s", basePath, toolName, entrypoint),
			content: string(entryData),
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
			continue // already uploaded
		}
		if strings.HasSuffix(fn, ".sh") || strings.HasSuffix(fn, ".py") || strings.HasSuffix(fn, ".json") {
			data, _ := os.ReadFile(filepath.Join(toolDir, fn))
			files = append(files, fileToUpload{
				path:    fmt.Sprintf("%s/%s/%s", basePath, toolName, fn),
				content: string(data),
			})
		}
	}

	// Upload each file via Gitea API
	apiBase := fmt.Sprintf("%s/api/v1/repos/%s/%s/contents", giteaURL, owner, repo)
	successCount := 0

	for _, f := range files {
		apiURL := fmt.Sprintf("%s/%s", apiBase, f.path)

		// Check if file already exists (get SHA)
		req, _ := http.NewRequest("GET", apiURL, nil)
		req.Header.Set("Authorization", "token "+token)
		resp, err := http.DefaultClient.Do(req)
		sha := ""
		if err == nil && resp.StatusCode == 200 {
			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)
			resp.Body.Close()
			if s, ok := result["sha"].(string); ok {
				sha = s
			}
		} else if resp != nil {
			resp.Body.Close()
		}

		// Create/update file
		payload := map[string]interface{}{
			"content":  base64.StdEncoding.EncodeToString([]byte(f.content)),
			"message":  fmt.Sprintf("feat(registry): publish %s - %s", toolName, f.path),
			"branch":   branch,
		}
		if sha != "" {
			payload["sha"] = sha
		}

		body, _ := json.Marshal(payload)
		req, _ = http.NewRequest("PUT", apiURL, strings.NewReader(string(body)))
		req.Header.Set("Authorization", "token "+token)
		req.Header.Set("Content-Type", "application/json")

		resp2, err2 := http.DefaultClient.Do(req)
		if err2 != nil {
			fmt.Fprintf(os.Stderr, "  ⚠ Failed to upload %s: %v\n", f.path, err2)
			continue
		}
		resp2.Body.Close()

		if resp2.StatusCode == 200 || resp2.StatusCode == 201 {
			fmt.Printf("  ✅ Uploaded: %s\n", f.path)
			successCount++
		} else {
			fmt.Fprintf(os.Stderr, "  ⚠ Upload %s returned HTTP %d\n", f.path, resp2.StatusCode)
		}
	}

	if successCount > 0 {
		fmt.Printf("\n✅ Published '%s' (%d/%d files)\n", toolName, successCount, len(files))
		fmt.Printf("   View: %s/%s/%s/src/branch/%s/%s/%s\n", giteaURL, owner, repo, branch, basePath, toolName)
	} else {
		fmt.Fprintf(os.Stderr, "\n❌ Publish failed\n")
		os.Exit(1)
	}
}


