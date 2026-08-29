package core

import "time"

type Workspace struct {
	Path string `json:"path"`
}

type Environment struct {
	Name       string    `json:"name"`
	Workspace  Workspace `json:"workspace"`
	RuntimeRef string    `json:"runtime_ref"`
	CreatedAt  time.Time `json:"created_at"`
}

type EnvironmentSpec struct {
	Name          string
	WorkspacePath string
}

type EnvironmentRuntimeSpec struct {
	Name          string
	WorkspacePath string
}

type EnvironmentRuntime struct {
	Ref string
}

type ExecutionRequest struct {
	Argv []string
}

type ExecutionResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}
