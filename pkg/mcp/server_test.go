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

package mcp_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"sigs.k8s.io/kubebuilder/v4/pkg/mcp"
)

func TestMCP(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "mcp suite")
}

// connectTestClient builds the Kubebuilder MCP server and returns a connected
// SDK client session backed by an in-memory transport. The caller must call
// session.Close() when done.
func connectTestClient(srv *mcp.Server) (*sdkmcp.ClientSession, error) {
	sdkSrv := srv.Build()

	t1, t2 := sdkmcp.NewInMemoryTransports()
	ctx := context.Background()

	if _, err := sdkSrv.Connect(ctx, t1, nil); err != nil {
		return nil, err
	}

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	return client.Connect(ctx, t2, nil)
}

const minimalProjectFile = `version: "3"
domain: example.com
repo: github.com/example/op
layout:
- go.kubebuilder.io/v4
resources:
- api:
    crdVersion: v1
    namespaced: true
  controller: true
  group: widgets
  kind: Widget
  plural: widgets
  version: v1alpha1
`

var _ = Describe("Server", func() {
	var (
		tmpDir  string
		session *sdkmcp.ClientSession
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "mcp-server-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if session != nil {
			Expect(session.Close()).To(Succeed())
			session = nil
		}
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	// makeSession builds a server for the given project directory and connects a test client.
	makeSession := func(projectDir string) *sdkmcp.ClientSession {
		srv := mcp.NewServer(
			mcp.WithVersion("v4.0.0-test"),
			mcp.WithProjectDir(projectDir),
		)
		sess, err := connectTestClient(srv)
		Expect(err).NotTo(HaveOccurred())
		return sess
	}

	Context("resources/list", func() {
		It("returns the expected resource URIs", func() {
			session = makeSession(tmpDir)
			ctx := context.Background()

			result, err := session.ListResources(ctx, nil)
			Expect(err).NotTo(HaveOccurred())

			var uris []string
			for _, r := range result.Resources {
				uris = append(uris, r.URI)
			}
			Expect(uris).To(ContainElements(
				"kubebuilder://version",
				"kubebuilder://project/config",
				"kubebuilder://project/apis",
				"kubebuilder://project/plugins",
			))
		})
	})

	Context("resources/read kubebuilder://version", func() {
		It("returns the version string", func() {
			session = makeSession(tmpDir)
			ctx := context.Background()

			result, err := session.ReadResource(ctx, &sdkmcp.ReadResourceParams{
				URI: "kubebuilder://version",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Contents).To(HaveLen(1))
			Expect(result.Contents[0].Text).To(ContainSubstring("v4.0.0-test"))
		})
	})

	Context("resources/read project resources", func() {
		BeforeEach(func() {
			Expect(os.WriteFile(filepath.Join(tmpDir, "PROJECT"), []byte(minimalProjectFile), 0o600)).To(Succeed())
		})

		It("returns project/config", func() {
			session = makeSession(tmpDir)
			ctx := context.Background()

			result, err := session.ReadResource(ctx, &sdkmcp.ReadResourceParams{
				URI: "kubebuilder://project/config",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Contents[0].Text).To(ContainSubstring("example.com"))
			Expect(result.Contents[0].Text).To(ContainSubstring("go.kubebuilder.io/v4"))
		})

		It("returns project/apis", func() {
			session = makeSession(tmpDir)
			ctx := context.Background()

			result, err := session.ReadResource(ctx, &sdkmcp.ReadResourceParams{
				URI: "kubebuilder://project/apis",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Contents[0].Text).To(ContainSubstring("Widget"))
			Expect(result.Contents[0].Text).To(ContainSubstring("widgets"))
		})

		It("returns project/plugins", func() {
			session = makeSession(tmpDir)
			ctx := context.Background()

			result, err := session.ReadResource(ctx, &sdkmcp.ReadResourceParams{
				URI: "kubebuilder://project/plugins",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Contents[0].Text).To(ContainSubstring("go.kubebuilder.io/v4"))
		})
	})

	Context("resources/read error cases", func() {
		It("returns an error for project resources when no PROJECT file exists", func() {
			session = makeSession(tmpDir)
			ctx := context.Background()

			_, err := session.ReadResource(ctx, &sdkmcp.ReadResourceParams{
				URI: "kubebuilder://project/config",
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("PROJECT"))
		})
	})

	Context("prompts/list", func() {
		It("returns the expected prompt names", func() {
			session = makeSession(tmpDir)
			ctx := context.Background()

			result, err := session.ListPrompts(ctx, nil)
			Expect(err).NotTo(HaveOccurred())

			var names []string
			for _, p := range result.Prompts {
				names = append(names, p.Name)
			}
			Expect(names).To(ContainElements("reconcile-best-practices", "project-summary"))
		})
	})

	Context("prompts/get", func() {
		It("returns reconcile-best-practices with version info and key content", func() {
			session = makeSession(tmpDir)
			ctx := context.Background()

			result, err := session.GetPrompt(ctx, &sdkmcp.GetPromptParams{
				Name: "reconcile-best-practices",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Messages).To(HaveLen(1))
			text := result.Messages[0].Content.(*sdkmcp.TextContent).Text
			Expect(text).To(ContainSubstring("v4.0.0-test"))
			Expect(text).To(ContainSubstring("Idempotency"))
		})

		It("returns project-summary", func() {
			session = makeSession(tmpDir)
			ctx := context.Background()

			result, err := session.GetPrompt(ctx, &sdkmcp.GetPromptParams{
				Name: "project-summary",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Messages).NotTo(BeEmpty())
		})

		It("returns an error for an unknown prompt", func() {
			session = makeSession(tmpDir)
			ctx := context.Background()

			_, err := session.GetPrompt(ctx, &sdkmcp.GetPromptParams{
				Name: "nonexistent",
			})
			Expect(err).To(HaveOccurred())
		})
	})
})
