package publish

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// gitHubBackend publishes tools to GitHub via the Contents API.
type gitHubBackend struct {
	cfg Config
}

func newGitHubBackend(cfg Config) (*gitHubBackend, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.github.com"
	}
	if cfg.Branch == "" {
		cfg.Branch = "main"
	}
	if cfg.BasePath == "" {
		cfg.BasePath = "registry/tools"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &gitHubBackend{cfg: cfg}, nil
}

func (g *gitHubBackend) Name() string { return "github" }

func (g *gitHubBackend) Upload(path string, content []byte) error {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/contents/%s",
		g.cfg.BaseURL, g.cfg.Owner, g.cfg.Repo, path)

	// Check if file exists to get SHA
	sha := ""
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("Authorization", "Bearer "+g.cfg.Token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if resp, err := http.DefaultClient.Do(req); err == nil && resp.StatusCode == 200 {
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		if s, ok := result["sha"].(string); ok {
			sha = s
		}
	} else if resp != nil {
		resp.Body.Close()
	}

	// Create/update file via Contents API
	payload := map[string]interface{}{
		"message": fmt.Sprintf("feat(registry): publish %s", path),
		"content": base64.StdEncoding.EncodeToString(content),
		"branch":  g.cfg.Branch,
	}
	if sha != "" {
		payload["sha"] = sha
	}

	body, _ := json.Marshal(payload)
	req, _ = http.NewRequest("PUT", apiURL, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+g.cfg.Token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("github: upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return fmt.Errorf("github: upload returned HTTP %d", resp.StatusCode)
	}

	return nil
}
