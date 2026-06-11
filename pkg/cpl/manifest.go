// Package cpl defines the core CPL (Containerized Pipeline Language) types
// used throughout UniCLI OS for manifest representation.
package cpl

// CPLManifest represents a complete CPL tool manifest.
type CPLManifest struct {
	CPLVersion  string      `json:"cpl_version"`
	Name        string      `json:"name"`
	Version     string      `json:"version"`
	Description string      `json:"description"`
	Author      string      `json:"author"`
	Inputs      []CPLInput  `json:"inputs"`
	Outputs     []CPLOutput `json:"outputs"`
	Resources   CPLResource `json:"resources"`
	Image       CPLImage    `json:"image"`
}

// CPLInput describes a single input parameter for a CPL tool.
type CPLInput struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default"`
	Description string      `json:"description"`
	Flag        string      `json:"flag"`
	Position    int         `json:"position"`
}

// CPLOutput describes a single output produced by a CPL tool.
type CPLOutput struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	Description    string `json:"description"`
	CaptureStdout  bool   `json:"capture_stdout"`
}

// CPLResource describes the resource requirements for running a CPL tool.
type CPLResource struct {
	CPU     float64 `json:"cpu"`
	Memory  int     `json:"memory"`
	Network bool    `json:"network"`
	GPU     bool    `json:"gpu"`
	Timeout int     `json:"timeout"`
	Disk    int     `json:"disk"`
}

// CPLImage describes the container image used to run a CPL tool.
type CPLImage struct {
	Ref        string `json:"ref"`
	Entrypoint string `json:"entrypoint"`
	Workdir    string `json:"workdir"`
	User       string `json:"user"`
}
