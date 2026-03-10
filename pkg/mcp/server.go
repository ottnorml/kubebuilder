/*
Copyright 2025 The Kubernetes Authors.

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
// the Kubebuilder CLI.  It exposes Kubebuilder project metadata and
// operator-development guidance over the MCP stdio transport so that AI
// assistants can discover and use Kubebuilder context directly.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"sigs.k8s.io/kubebuilder/v4/pkg/mcp/project"
	"sigs.k8s.io/kubebuilder/v4/pkg/mcp/prompts"
)

// protocolVersion is the MCP protocol version advertised by this server.
const protocolVersion = "2024-11-05"

// rpcRequest is a minimal JSON-RPC 2.0 request.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// rpcResponse is a JSON-RPC 2.0 response.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcError is a JSON-RPC 2.0 error object.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Standard JSON-RPC 2.0 error codes.
const (
	codeParseError     = -32700
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
)

// Server is a read-only MCP server that exposes Kubebuilder resources and
// prompts over the stdio transport.
type Server struct {
	// version is the Kubebuilder version string surfaced in resources/prompts.
	version string
	// projectDir is the directory to load project context from.
	projectDir string
	// in is the stream to read JSON-RPC messages from (default: os.Stdin).
	in io.Reader
	// out is the stream to write JSON-RPC messages to (default: os.Stdout).
	out io.Writer
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

// WithIO overrides the default stdin/stdout streams (useful for testing).
func WithIO(in io.Reader, out io.Writer) Option {
	return func(s *Server) {
		s.in = in
		s.out = out
	}
}

// NewServer creates a new MCP Server with the provided options.
func NewServer(opts ...Option) *Server {
	s := &Server{
		version: "(devel)",
		in:      os.Stdin,
		out:     os.Stdout,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Run starts the MCP server and serves requests until ctx is cancelled or the
// input stream is exhausted.
func (s *Server) Run(ctx context.Context) error {
	slog.Info("kubebuilder MCP server starting", "transport", "stdio", "version", s.version)

	scanner := bufio.NewScanner(s.in)
	// Allow lines up to 4 MB (large projects may produce big JSON).
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("reading stdin: %w", err)
			}
			// EOF — client closed the connection.
			return nil
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req rpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.writeError(nil, codeParseError, "parse error: "+err.Error())
			continue
		}

		s.handle(req)
	}
}

// handle dispatches a single JSON-RPC request.
func (s *Server) handle(req rpcRequest) {
	// Notifications (no id) are silently acknowledged.
	isNotification := req.ID == nil || string(req.ID) == "null"

	switch req.Method {
	case "initialize":
		if isNotification {
			return
		}
		s.writeResult(req.ID, s.handleInitialize())

	case "initialized":
		// Client notification – nothing to do.

	case "ping":
		if isNotification {
			return
		}
		s.writeResult(req.ID, map[string]any{})

	case "resources/list":
		if isNotification {
			return
		}
		s.writeResult(req.ID, s.handleResourcesList())

	case "resources/read":
		if isNotification {
			return
		}
		result, err := s.handleResourcesRead(req.Params)
		if err != nil {
			s.writeError(req.ID, codeInvalidParams, err.Error())
			return
		}
		s.writeResult(req.ID, result)

	case "prompts/list":
		if isNotification {
			return
		}
		s.writeResult(req.ID, s.handlePromptsList())

	case "prompts/get":
		if isNotification {
			return
		}
		result, err := s.handlePromptsGet(req.Params)
		if err != nil {
			s.writeError(req.ID, codeInvalidParams, err.Error())
			return
		}
		s.writeResult(req.ID, result)

	default:
		if !isNotification {
			s.writeError(req.ID, codeMethodNotFound, fmt.Sprintf("method not found: %q", req.Method))
		}
	}
}

// --- initialize ---

type initializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    serverCapabilities `json:"capabilities"`
	ServerInfo      serverInfo         `json:"serverInfo"`
}

type serverCapabilities struct {
	Resources resourceCapability `json:"resources"`
	Prompts   promptCapability   `json:"prompts"`
}

type resourceCapability struct{}
type promptCapability struct{}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (s *Server) handleInitialize() initializeResult {
	return initializeResult{
		ProtocolVersion: protocolVersion,
		Capabilities: serverCapabilities{
			Resources: resourceCapability{},
			Prompts:   promptCapability{},
		},
		ServerInfo: serverInfo{
			Name:    "kubebuilder",
			Version: s.version,
		},
	}
}

// --- resources ---

type resourceDescriptor struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MIMEType    string `json:"mimeType"`
}

type resourcesListResult struct {
	Resources []resourceDescriptor `json:"resources"`
}

func (s *Server) handleResourcesList() resourcesListResult {
	return resourcesListResult{
		Resources: []resourceDescriptor{
			{
				URI:         "kubebuilder://version",
				Name:        "Kubebuilder Version",
				Description: "Current Kubebuilder build information",
				MIMEType:    "application/json",
			},
			{
				URI:         "kubebuilder://project/config",
				Name:        "Project Configuration",
				Description: "Contents of the PROJECT file summarized as JSON",
				MIMEType:    "application/json",
			},
			{
				URI:         "kubebuilder://project/apis",
				Name:        "Project APIs",
				Description: "List of API resources defined in the project",
				MIMEType:    "application/json",
			},
			{
				URI:         "kubebuilder://project/plugins",
				Name:        "Project Plugins",
				Description: "Active plugin chain configured in the project",
				MIMEType:    "application/json",
			},
		},
	}
}

type resourceContent struct {
	URI      string `json:"uri"`
	MIMEType string `json:"mimeType"`
	Text     string `json:"text"`
}

type resourceReadResult struct {
	Contents []resourceContent `json:"contents"`
}

type resourceReadParams struct {
	URI string `json:"uri"`
}

func (s *Server) handleResourcesRead(raw json.RawMessage) (resourceReadResult, error) {
	var params resourceReadParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return resourceReadResult{}, fmt.Errorf("invalid params: %w", err)
	}

	switch params.URI {
	case "kubebuilder://version":
		return s.resourceVersion()
	case "kubebuilder://project/config":
		return s.resourceProjectConfig()
	case "kubebuilder://project/apis":
		return s.resourceProjectAPIs()
	case "kubebuilder://project/plugins":
		return s.resourceProjectPlugins()
	default:
		return resourceReadResult{}, fmt.Errorf("unknown resource URI: %q", params.URI)
	}
}

func (s *Server) resourceVersion() (resourceReadResult, error) {
	payload := map[string]string{"version": s.version}
	text, err := marshalJSON(payload)
	if err != nil {
		return resourceReadResult{}, err
	}
	return resourceReadResult{Contents: []resourceContent{{
		URI: "kubebuilder://version", MIMEType: "application/json", Text: text,
	}}}, nil
}

func (s *Server) resourceProjectConfig() (resourceReadResult, error) {
	ctx, err := project.LoadContext(s.projectDir)
	if err != nil {
		return resourceReadResult{}, err
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
		return resourceReadResult{}, err
	}
	return resourceReadResult{Contents: []resourceContent{{
		URI: "kubebuilder://project/config", MIMEType: "application/json", Text: text,
	}}}, nil
}

func (s *Server) resourceProjectAPIs() (resourceReadResult, error) {
	ctx, err := project.LoadContext(s.projectDir)
	if err != nil {
		return resourceReadResult{}, err
	}

	text, err := marshalJSON(ctx.APIs)
	if err != nil {
		return resourceReadResult{}, err
	}
	return resourceReadResult{Contents: []resourceContent{{
		URI: "kubebuilder://project/apis", MIMEType: "application/json", Text: text,
	}}}, nil
}

func (s *Server) resourceProjectPlugins() (resourceReadResult, error) {
	ctx, err := project.LoadContext(s.projectDir)
	if err != nil {
		return resourceReadResult{}, err
	}

	payload := map[string]any{
		"configured": ctx.PluginChain,
	}
	text, err := marshalJSON(payload)
	if err != nil {
		return resourceReadResult{}, err
	}
	return resourceReadResult{Contents: []resourceContent{{
		URI: "kubebuilder://project/plugins", MIMEType: "application/json", Text: text,
	}}}, nil
}

// --- prompts ---

type promptDescriptor struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type promptsListResult struct {
	Prompts []promptDescriptor `json:"prompts"`
}

func (s *Server) handlePromptsList() promptsListResult {
	return promptsListResult{
		Prompts: []promptDescriptor{
			{
				Name:        "reconcile-best-practices",
				Description: "Best practices for implementing a Kubernetes controller Reconcile function",
			},
			{
				Name:        "project-summary",
				Description: "Summary of the current Kubebuilder project",
			},
		},
	}
}

type promptMessage struct {
	Role    string        `json:"role"`
	Content promptContent `json:"content"`
}

type promptContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type promptGetResult struct {
	Description string          `json:"description"`
	Messages    []promptMessage `json:"messages"`
}

type promptGetParams struct {
	Name string `json:"name"`
}

func (s *Server) handlePromptsGet(raw json.RawMessage) (promptGetResult, error) {
	var params promptGetParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return promptGetResult{}, fmt.Errorf("invalid params: %w", err)
	}

	switch params.Name {
	case "reconcile-best-practices":
		text, err := prompts.RenderReconcileBestPractices(s.version)
		if err != nil {
			return promptGetResult{}, fmt.Errorf("rendering prompt: %w", err)
		}
		return promptGetResult{
			Description: "Best practices for implementing a Kubernetes controller Reconcile function",
			Messages: []promptMessage{
				{Role: "user", Content: promptContent{Type: "text", Text: text}},
			},
		}, nil

	case "project-summary":
		ctx, _ := project.LoadContext(s.projectDir) // nil ctx is acceptable
		text, err := prompts.RenderProjectSummary(s.version, ctx)
		if err != nil {
			return promptGetResult{}, fmt.Errorf("rendering prompt: %w", err)
		}
		return promptGetResult{
			Description: "Summary of the current Kubebuilder project",
			Messages: []promptMessage{
				{Role: "user", Content: promptContent{Type: "text", Text: text}},
			},
		}, nil

	default:
		return promptGetResult{}, fmt.Errorf("unknown prompt: %q", params.Name)
	}
}

// --- transport helpers ---

func (s *Server) writeResult(id json.RawMessage, result any) {
	s.write(rpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

func (s *Server) writeError(id json.RawMessage, code int, message string) {
	s.write(rpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: message},
	})
}

func (s *Server) write(resp rpcResponse) {
	b, err := json.Marshal(resp)
	if err != nil {
		slog.Error("failed to marshal JSON-RPC response", "error", err)
		return
	}
	if _, err := fmt.Fprintln(s.out, string(b)); err != nil {
		slog.Error("failed to write JSON-RPC response", "error", err)
	}
}

// marshalJSON is a thin wrapper that returns a JSON string.
func marshalJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshalling resource payload: %w", err)
	}
	return string(b), nil
}
