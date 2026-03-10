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
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/kubebuilder/v4/pkg/mcp"
)

func TestMCP(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "mcp suite")
}

// callServer sends a newline-delimited sequence of JSON-RPC requests to the
// server and returns all response lines as a slice of decoded maps.
func callServer(projectDir string, requestLines ...string) ([]map[string]any, error) {
	in := strings.NewReader(strings.Join(requestLines, "\n") + "\n")
	var out bytes.Buffer

	s := mcp.NewServer(
		mcp.WithVersion("v4.0.0-test"),
		mcp.WithProjectDir(projectDir),
		mcp.WithIO(in, &out),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := s.Run(ctx); err != nil {
		return nil, err
	}

	var results []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			return nil, err
		}
		results = append(results, m)
	}
	return results, nil
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
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "mcp-server-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	Context("initialize", func() {
		It("returns protocol version and capabilities", func() {
			resps, err := callServer(tmpDir,
				`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(resps).To(HaveLen(1))

			resp := resps[0]
			result := resp["result"].(map[string]any)
			Expect(result["protocolVersion"]).To(Equal("2024-11-05"))
			info := result["serverInfo"].(map[string]any)
			Expect(info["name"]).To(Equal("kubebuilder"))
			Expect(info["version"]).To(Equal("v4.0.0-test"))
		})
	})

	Context("ping", func() {
		It("returns empty result", func() {
			resps, err := callServer(tmpDir,
				`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(resps).To(HaveLen(1))
			result := resps[0]["result"].(map[string]any)
			Expect(result).To(BeEmpty())
		})
	})

	Context("resources/list", func() {
		It("returns the expected resource URIs", func() {
			resps, err := callServer(tmpDir,
				`{"jsonrpc":"2.0","id":3,"method":"resources/list"}`,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(resps).To(HaveLen(1))

			result := resps[0]["result"].(map[string]any)
			resources := result["resources"].([]any)
			var uris []string
			for _, r := range resources {
				uris = append(uris, r.(map[string]any)["uri"].(string))
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
		It("returns version string", func() {
			resps, err := callServer(tmpDir,
				`{"jsonrpc":"2.0","id":4,"method":"resources/read","params":{"uri":"kubebuilder://version"}}`,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(resps).To(HaveLen(1))

			result := resps[0]["result"].(map[string]any)
			contents := result["contents"].([]any)
			Expect(contents).To(HaveLen(1))
			text := contents[0].(map[string]any)["text"].(string)
			Expect(text).To(ContainSubstring("v4.0.0-test"))
		})
	})

	Context("resources/read project resources", func() {
		BeforeEach(func() {
			Expect(os.WriteFile(filepath.Join(tmpDir, "PROJECT"), []byte(minimalProjectFile), 0o600)).To(Succeed())
		})

		It("returns project/config", func() {
			resps, err := callServer(tmpDir,
				`{"jsonrpc":"2.0","id":5,"method":"resources/read","params":{"uri":"kubebuilder://project/config"}}`,
			)
			Expect(err).NotTo(HaveOccurred())
			result := resps[0]["result"].(map[string]any)
			text := result["contents"].([]any)[0].(map[string]any)["text"].(string)
			Expect(text).To(ContainSubstring("example.com"))
			Expect(text).To(ContainSubstring("go.kubebuilder.io/v4"))
		})

		It("returns project/apis", func() {
			resps, err := callServer(tmpDir,
				`{"jsonrpc":"2.0","id":6,"method":"resources/read","params":{"uri":"kubebuilder://project/apis"}}`,
			)
			Expect(err).NotTo(HaveOccurred())
			result := resps[0]["result"].(map[string]any)
			text := result["contents"].([]any)[0].(map[string]any)["text"].(string)
			Expect(text).To(ContainSubstring("Widget"))
			Expect(text).To(ContainSubstring("widgets"))
		})

		It("returns project/plugins", func() {
			resps, err := callServer(tmpDir,
				`{"jsonrpc":"2.0","id":7,"method":"resources/read","params":{"uri":"kubebuilder://project/plugins"}}`,
			)
			Expect(err).NotTo(HaveOccurred())
			result := resps[0]["result"].(map[string]any)
			text := result["contents"].([]any)[0].(map[string]any)["text"].(string)
			Expect(text).To(ContainSubstring("go.kubebuilder.io/v4"))
		})
	})

	Context("resources/read error cases", func() {
		It("returns error for unknown URI", func() {
			resps, err := callServer(tmpDir,
				`{"jsonrpc":"2.0","id":8,"method":"resources/read","params":{"uri":"kubebuilder://unknown"}}`,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(resps[0]["error"]).NotTo(BeNil())
		})

		It("returns error for project resources when no PROJECT file exists", func() {
			resps, err := callServer(tmpDir,
				`{"jsonrpc":"2.0","id":9,"method":"resources/read","params":{"uri":"kubebuilder://project/config"}}`,
			)
			Expect(err).NotTo(HaveOccurred())
			errObj := resps[0]["error"].(map[string]any)
			Expect(errObj["message"]).To(ContainSubstring("PROJECT"))
		})
	})

	Context("prompts/list", func() {
		It("returns expected prompt names", func() {
			resps, err := callServer(tmpDir,
				`{"jsonrpc":"2.0","id":10,"method":"prompts/list"}`,
			)
			Expect(err).NotTo(HaveOccurred())
			result := resps[0]["result"].(map[string]any)
			ps := result["prompts"].([]any)
			var names []string
			for _, p := range ps {
				names = append(names, p.(map[string]any)["name"].(string))
			}
			Expect(names).To(ContainElements("reconcile-best-practices", "project-summary"))
		})
	})

	Context("prompts/get", func() {
		It("returns reconcile-best-practices with version info", func() {
			resps, err := callServer(tmpDir,
				`{"jsonrpc":"2.0","id":11,"method":"prompts/get","params":{"name":"reconcile-best-practices"}}`,
			)
			Expect(err).NotTo(HaveOccurred())
			result := resps[0]["result"].(map[string]any)
			msgs := result["messages"].([]any)
			Expect(msgs).To(HaveLen(1))
			text := msgs[0].(map[string]any)["content"].(map[string]any)["text"].(string)
			Expect(text).To(ContainSubstring("v4.0.0-test"))
			Expect(text).To(ContainSubstring("Idempotency"))
		})

		It("returns project-summary", func() {
			resps, err := callServer(tmpDir,
				`{"jsonrpc":"2.0","id":12,"method":"prompts/get","params":{"name":"project-summary"}}`,
			)
			Expect(err).NotTo(HaveOccurred())
			result := resps[0]["result"].(map[string]any)
			Expect(result["messages"]).NotTo(BeEmpty())
		})

		It("returns error for unknown prompt", func() {
			resps, err := callServer(tmpDir,
				`{"jsonrpc":"2.0","id":13,"method":"prompts/get","params":{"name":"nonexistent"}}`,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(resps[0]["error"]).NotTo(BeNil())
		})
	})

	Context("unknown method", func() {
		It("returns method-not-found error", func() {
			resps, err := callServer(tmpDir,
				`{"jsonrpc":"2.0","id":14,"method":"tools/call","params":{}}`,
			)
			Expect(err).NotTo(HaveOccurred())
			errObj := resps[0]["error"].(map[string]any)
			Expect(errObj["code"]).To(BeEquivalentTo(-32601))
		})
	})

	Context("notifications (no id)", func() {
		It("does not produce a response for initialized notification", func() {
			resps, err := callServer(tmpDir,
				`{"jsonrpc":"2.0","method":"initialized"}`,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(resps).To(BeEmpty())
		})
	})
})
