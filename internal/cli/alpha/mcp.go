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

package alpha

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"sigs.k8s.io/kubebuilder/v4/internal/cli/version"
	"sigs.k8s.io/kubebuilder/v4/pkg/mcp"
)

// NewMCPCommand returns the `kubebuilder alpha mcp` command, which starts a
// read-only Model Context Protocol (MCP) server over stdio so that AI
// assistants can discover Kubebuilder resources and prompts.
func NewMCPCommand() *cobra.Command {
	var (
		transport  string
		projectDir string
	)

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run an MCP server for AI-assisted operator development (alpha)",
		Long: `Run a read-only Model Context Protocol (MCP) server over stdio.

The server exposes Kubebuilder project metadata and operator-development
guidance so that AI assistants (e.g. Claude, Cursor, Copilot) can discover
Kubebuilder context and best practices directly from the binary.

EXPERIMENTAL: this command is read-only by default. Mutating tools are not
exposed. The protocol and resource URIs may change in future releases.

Available resources:
  kubebuilder://version           Kubebuilder build information
  kubebuilder://project/config    Parsed PROJECT file metadata
  kubebuilder://project/apis      API resources defined in the project
  kubebuilder://project/plugins   Active plugin chain

Available prompts:
  reconcile-best-practices        Guidance for writing Reconcile functions
  project-summary                 Summary of the current project

Run from the root of a Kubebuilder project to enable project-scoped resources.
`,
		Example: `  # Start the MCP server (stdio transport)
  kubebuilder alpha mcp

  # Start the MCP server from a specific project directory
  kubebuilder alpha mcp --project-dir /path/to/my-operator
`,
		RunE: func(_ *cobra.Command, _ []string) error {
			if transport != "stdio" {
				return fmt.Errorf("unsupported transport %q: only \"stdio\" is supported", transport)
			}

			v := version.New()
			srv := mcp.NewServer(
				mcp.WithVersion(v.GetKubeBuilderVersion()),
				mcp.WithProjectDir(projectDir),
			)

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			return srv.Run(ctx)
		},
	}

	cmd.Flags().StringVar(&transport, "transport", "stdio",
		"transport to use for the MCP server (only \"stdio\" is supported)")
	cmd.Flags().StringVar(&projectDir, "project-dir", "",
		"directory containing the Kubebuilder PROJECT file (defaults to the current directory)")

	return cmd
}
