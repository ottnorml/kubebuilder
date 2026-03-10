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

// Package prompts provides embedded, version-aware prompt templates for
// the Kubebuilder MCP server.
package prompts

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"

	"sigs.k8s.io/kubebuilder/v4/pkg/mcp/project"
)

//go:embed reconcile_best_practices.md.tmpl
var reconcileBestPracticesTmpl string

//go:embed project_summary.md.tmpl
var projectSummaryTmpl string

// reconcileBestPracticesData holds template data for the reconcile prompt.
type reconcileBestPracticesData struct {
	Version string
}

// projectSummaryData holds template data for the project-summary prompt.
type projectSummaryData struct {
	Version        string
	ProjectName    string
	Domain         string
	Repository     string
	ProjectVersion string
	MultiGroup     bool
	PluginChain    []string
	APIs           []project.APIInfo
}

// templateFuncs provides helper functions available in prompt templates.
var templateFuncs = template.FuncMap{
	"boolIcon": func(v bool) string {
		if v {
			return "✓"
		}
		return "✗"
	},
	"webhookSummary": func(w *project.WebhookInfo) string {
		if w == nil {
			return "none"
		}
		var parts []string
		if w.Defaulting {
			parts = append(parts, "default")
		}
		if w.Validation {
			parts = append(parts, "validate")
		}
		if w.Conversion {
			parts = append(parts, "convert")
		}
		if len(parts) == 0 {
			return "none"
		}
		result := parts[0]
		for i := 1; i < len(parts); i++ {
			result += ", " + parts[i]
		}
		return result
	},
}

// RenderReconcileBestPractices renders the reconcile-best-practices prompt for
// the given Kubebuilder version string.
func RenderReconcileBestPractices(version string) (string, error) {
	return render(reconcileBestPracticesTmpl, "reconcile-best-practices", reconcileBestPracticesData{
		Version: version,
	})
}

// RenderProjectSummary renders the project-summary prompt for the given
// version and project context. ctx may be nil when running outside a project.
func RenderProjectSummary(version string, ctx *project.Context) (string, error) {
	data := projectSummaryData{
		Version: version,
	}
	if ctx != nil {
		data.ProjectName = ctx.ProjectName
		data.Domain = ctx.Domain
		data.Repository = ctx.Repository
		data.ProjectVersion = ctx.ProjectVersion
		data.MultiGroup = ctx.MultiGroup
		data.PluginChain = ctx.PluginChain
		data.APIs = ctx.APIs
	}
	return render(projectSummaryTmpl, "project-summary", data)
}

// render executes a named template with data and returns the result.
func render(tmplStr, name string, data any) (string, error) {
	t, err := template.New(name).Funcs(templateFuncs).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parsing prompt template %q: %w", name, err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("rendering prompt template %q: %w", name, err)
	}

	return buf.String(), nil
}
