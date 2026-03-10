/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package mcp implements a read-only Model Context Protocol (MCP) server for
// the Kubebuilder CLI. It exposes Kubebuilder project metadata and
// operator-development guidance over the MCP stdio transport so that AI
// assistants can discover and use Kubebuilder context directly.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"sigs.k8s.io/kubebuilder/v4/pkg/mcp/project"
	"sigs.k8s.io/kubebuilder/v4/pkg/mcp/prompts"
)

// Server holds the configuration for the Kubebuilder MCP server.
type Server struct {
	// version is the Kubebuilder version string surfaced in resources/prompts.
	version string
	// projectDir is the directory to load project context from.
	projectDir string
}

// Option is a functional option for Server.
type Option func(*Server)

// WithVersion sets the Kubebuilder version string reported by the server.
func WithVersion(v string) Option {
	return func(s *Server) { s.version = v }
}

// WithProjectDir sets the directory from which the project context is loaded.
func WithProjectDir(dir string) Option {
	return func(s *Server) { s.projectDir = dir }
}

// NewServer creates a new MCP Server with the provided options.
func NewServer(opts ...Option) *Server {
	s := &Server{version: "(devel)"}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Run starts the MCP server over the stdio transport and blocks until ctx is
// cancelled or the client disconnects.
func (s *Server) Run(ctx context.Context) error {
	slog.Info("starting Kubebuilder MCP server", "transport", "stdio", "version", s.version)
	return s.Build().Run(ctx, &sdkmcp.StdioTransport{})
}

// Build constructs and returns the underlying SDK server with all resources and
// prompts registered. It is used by Run and also exposed for testing via an
// in-memory transport.
func (s *Server) Build() *sdkmcp.Server {
	srv := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "kubebuilder",
		Version: s.version,
	}, nil)
	s.registerResources(srv)
	s.registerPrompts(srv)
	return srv
}

// --- resources ---

func (s *Server) registerResources(srv *sdkmcp.Server) {
	srv.AddResource(&sdkmcp.Resource{
		URI:         "kubebuilder://version",
		Name:        "Kubebuilder Version",
		Description: "Current Kubebuilder build information",
		MIMEType:    "application/json",
	}, s.handleVersion)

	srv.AddResource(&sdkmcp.Resource{
		URI:         "kubebuilder://project/config",
		Name:        "Project Configuration",
		Description: "Contents of the PROJECT file summarized as JSON",
		MIMEType:    "application/json",
	}, s.handleProjectConfig)

	srv.AddResource(&sdkmcp.Resource{
		URI:         "kubebuilder://project/apis",
		Name:        "Project APIs",
		Description: "List of API resources defined in the project",
		MIMEType:    "application/json",
	}, s.handleProjectAPIs)

	srv.AddResource(&sdkmcp.Resource{
		URI:         "kubebuilder://project/plugins",
		Name:        "Project Plugins",
		Description: "Active plugin chain configured in the project",
		MIMEType:    "application/json",
	}, s.handleProjectPlugins)
}

func (s *Server) handleVersion(_ context.Context, _ *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
	text, err := marshalJSON(map[string]string{"version": s.version})
	if err != nil {
		return nil, err
	}
	return resourceResult("kubebuilder://version", text), nil
}

func (s *Server) handleProjectConfig(_ context.Context, _ *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
	ctx, err := project.LoadContext(s.projectDir)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"version":     ctx.ProjectVersion,
		"domain":      ctx.Domain,
		"repo":        ctx.Repository,
		"projectName": ctx.ProjectName,
		"layout":      ctx.PluginChain,
		"multiGroup":  ctx.MultiGroup,
		"cliVersion":  ctx.CliVersion,
	}
	text, err := marshalJSON(payload)
	if err != nil {
		return nil, err
	}
	return resourceResult("kubebuilder://project/config", text), nil
}

func (s *Server) handleProjectAPIs(_ context.Context, _ *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
	ctx, err := project.LoadContext(s.projectDir)
	if err != nil {
		return nil, err
	}
	text, err := marshalJSON(ctx.APIs)
	if err != nil {
		return nil, err
	}
	return resourceResult("kubebuilder://project/apis", text), nil
}

func (s *Server) handleProjectPlugins(_ context.Context, _ *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
	ctx, err := project.LoadContext(s.projectDir)
	if err != nil {
		return nil, err
	}
	text, err := marshalJSON(map[string]any{"configured": ctx.PluginChain})
	if err != nil {
		return nil, err
	}
	return resourceResult("kubebuilder://project/plugins", text), nil
}

// resourceResult is a convenience constructor for a single-entry ReadResourceResult.
func resourceResult(uri, text string) *sdkmcp.ReadResourceResult {
	return &sdkmcp.ReadResourceResult{
		Contents: []*sdkmcp.ResourceContents{{
			URI:      uri,
			MIMEType: "application/json",
			Text:     text,
		}},
	}
}

// --- prompts ---

func (s *Server) registerPrompts(srv *sdkmcp.Server) {
	srv.AddPrompt(&sdkmcp.Prompt{
		Name:        "reconcile-best-practices",
		Description: "Best practices for implementing a Kubernetes controller Reconcile function",
	}, s.handleReconcileBestPractices)

	srv.AddPrompt(&sdkmcp.Prompt{
		Name:        "project-summary",
		Description: "Summary of the current Kubebuilder project",
	}, s.handleProjectSummary)
}

func (s *Server) handleReconcileBestPractices(_ context.Context, _ *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
	text, err := prompts.RenderReconcileBestPractices(s.version)
	if err != nil {
		return nil, fmt.Errorf("rendering prompt: %w", err)
	}
	return &sdkmcp.GetPromptResult{
		Description: "Best practices for implementing a Kubernetes controller Reconcile function",
		Messages: []*sdkmcp.PromptMessage{
			{Role: "user", Content: &sdkmcp.TextContent{Text: text}},
		},
	}, nil
}

func (s *Server) handleProjectSummary(_ context.Context, _ *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
	ctx, _ := project.LoadContext(s.projectDir) // nil ctx is acceptable; prompt renders without project
	text, err := prompts.RenderProjectSummary(s.version, ctx)
	if err != nil {
		return nil, fmt.Errorf("rendering prompt: %w", err)
	}
	return &sdkmcp.GetPromptResult{
		Description: "Summary of the current Kubebuilder project",
		Messages: []*sdkmcp.PromptMessage{
			{Role: "user", Content: &sdkmcp.TextContent{Text: text}},
		},
	}, nil
}

// --- helpers ---

// marshalJSON serialises v to a compact JSON string.
func marshalJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshaling resource payload: %w", err)
	}
	return string(b), nil
}
