// Package publish provides abstractions for publishing CPL tools to
// remote registries. Supports multiple backends (Gitea, GitHub, etc.).
package publish

// Backend defines the interface for publishing tools to a remote registry.
type Backend interface {
	// Name returns the backend name (e.g. "gitea", "github").
	Name() string

	// Upload uploads a file to the registry at the given path.
	// content is the raw file content (base64 encoding is handled internally).
	Upload(path string, content []byte) error
}

// Config holds common configuration for publish backends.
type Config struct {
	// BackendType is "gitea" or "github"
	BackendType string
	// BaseURL is the API endpoint (e.g. "https://api.github.com")
	BaseURL string
	// Token is the authentication token
	Token string
	// Owner is the repository owner (user or org)
	Owner string
	// Repo is the repository name
	Repo string
	// Branch is the target branch (default: "main")
	Branch string
	// BasePath is the directory within the repo (e.g. "registry/tools")
	BasePath string
}

// New creates a publish backend based on the config.
func New(cfg Config) (Backend, error) {
	switch cfg.BackendType {
	case "gitea":
		return newGiteaBackend(cfg)
	case "github":
		return newGitHubBackend(cfg)
	default:
		return newGiteaBackend(cfg) // default to Gitea
	}
}

// defaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		BackendType: "gitea",
		BaseURL:     "http://localhost:3000",
		Owner:       "local-desktop",
		Repo:        "unicli-os",
		Branch:      "main",
		BasePath:    "registry/tools",
	}
}
