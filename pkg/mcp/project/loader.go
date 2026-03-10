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

// Package project provides utilities for loading and summarizing a Kubebuilder
// project's configuration from the PROJECT file.
package project

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/afero"

	"sigs.k8s.io/kubebuilder/v4/pkg/config"
	"sigs.k8s.io/kubebuilder/v4/pkg/config/store/yaml"
	"sigs.k8s.io/kubebuilder/v4/pkg/machinery"
	"sigs.k8s.io/kubebuilder/v4/pkg/model/resource"

	// Register config version v3 so the store can decode it.
	_ "sigs.k8s.io/kubebuilder/v4/pkg/config/v3"
)

// APIInfo summarises a single API/GVK registered in the project.
type APIInfo struct {
	Group      string       `json:"group"`
	Version    string       `json:"version"`
	Kind       string       `json:"kind"`
	Plural     string       `json:"plural,omitempty"`
	Controller bool         `json:"controller"`
	HasAPI     bool         `json:"hasAPI"`
	Namespaced bool         `json:"namespaced"`
	Webhooks   *WebhookInfo `json:"webhooks,omitempty"`
}

// WebhookInfo describes which webhooks are registered for a resource.
type WebhookInfo struct {
	Defaulting bool `json:"defaulting"`
	Validation bool `json:"validation"`
	Conversion bool `json:"conversion"`
}

// Context holds a summary of the current Kubebuilder project.
type Context struct {
	// RootDir is the directory containing the PROJECT file.
	RootDir string `json:"rootDir"`
	// ProjectVersion is the project configuration version (e.g. "3").
	ProjectVersion string `json:"projectVersion"`
	// Domain is the project domain.
	Domain string `json:"domain,omitempty"`
	// Repository is the Go module repository.
	Repository string `json:"repository,omitempty"`
	// ProjectName is the optional project name field.
	ProjectName string `json:"projectName,omitempty"`
	// PluginChain holds the active plugin layout keys.
	PluginChain []string `json:"pluginChain,omitempty"`
	// MultiGroup reports whether the project uses multi-group layout.
	MultiGroup bool `json:"multiGroup"`
	// APIs is the list of API resources defined in the project.
	APIs []APIInfo `json:"apis,omitempty"`
	// CliVersion is the Kubebuilder CLI version recorded in the PROJECT file.
	CliVersion string `json:"cliVersion,omitempty"`
}

// ErrNotProject is returned when the target directory does not contain a
// Kubebuilder PROJECT file.
type ErrNotProject struct {
	Dir string
}

func (e ErrNotProject) Error() string {
	return fmt.Sprintf("no Kubebuilder PROJECT file found in %q; run this command from a Kubebuilder project root", e.Dir)
}

// LoadContext loads and summarises the Kubebuilder project rooted at dir.
// It returns ErrNotProject when the directory does not contain a PROJECT file.
func LoadContext(dir string) (*Context, error) {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("unable to determine working directory: %w", err)
		}
	}

	// Resolve to an absolute path so error messages are clear.
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("unable to resolve path %q: %w", dir, err)
	}

	projectFile := filepath.Join(absDir, yaml.DefaultPath)
	if _, err := os.Stat(projectFile); os.IsNotExist(err) {
		return nil, ErrNotProject{Dir: absDir}
	}

	fs := machinery.Filesystem{FS: afero.NewBasePathFs(afero.NewOsFs(), absDir)}
	store := yaml.New(fs)
	if err := store.Load(); err != nil {
		return nil, fmt.Errorf("unable to load PROJECT file in %q: %w", absDir, err)
	}

	cfg := store.Config()
	return buildContext(absDir, cfg), nil
}

// buildContext converts a config.Config into a Context summary.
func buildContext(rootDir string, cfg config.Config) *Context {
	ctx := &Context{
		RootDir:        rootDir,
		ProjectVersion: cfg.GetVersion().String(),
		Domain:         cfg.GetDomain(),
		Repository:     cfg.GetRepository(),
		ProjectName:    cfg.GetProjectName(),
		PluginChain:    cfg.GetPluginChain(),
		MultiGroup:     cfg.IsMultiGroup(),
		CliVersion:     cfg.GetCliVersion(),
	}

	resources, err := cfg.GetResources()
	if err == nil {
		for _, r := range resources {
			ctx.APIs = append(ctx.APIs, summariseResource(r))
		}
	}

	return ctx
}

// summariseResource converts a resource.Resource to a minimal APIInfo.
func summariseResource(r resource.Resource) APIInfo {
	info := APIInfo{
		Group:      r.Group,
		Version:    r.Version,
		Kind:       r.Kind,
		Plural:     r.Plural,
		Controller: r.Controller,
	}

	if r.API != nil {
		info.HasAPI = !r.API.IsEmpty()
		info.Namespaced = r.API.Namespaced
	}

	if r.Webhooks != nil && !r.Webhooks.IsEmpty() {
		info.Webhooks = &WebhookInfo{
			Defaulting: r.Webhooks.Defaulting,
			Validation: r.Webhooks.Validation,
			Conversion: r.Webhooks.Conversion,
		}
	}

	return info
}
