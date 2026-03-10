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

package prompts_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/kubebuilder/v4/pkg/mcp/project"
	"sigs.k8s.io/kubebuilder/v4/pkg/mcp/prompts"
)

func TestPrompts(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "prompts suite")
}

var _ = Describe("RenderReconcileBestPractices", func() {
	It("renders without error and contains key sections", func() {
		out, err := prompts.RenderReconcileBestPractices("v4.0.0")
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring("v4.0.0"))
		Expect(out).To(ContainSubstring("Idempotency"))
		Expect(out).To(ContainSubstring("Finalizers"))
		Expect(out).To(ContainSubstring("Requeue"))
	})
})

var _ = Describe("RenderProjectSummary", func() {
	It("renders without error when ctx is nil", func() {
		out, err := prompts.RenderProjectSummary("v4.0.0", nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring("v4.0.0"))
	})

	It("renders project details when ctx is provided", func() {
		ctx := &project.Context{
			RootDir:        "/tmp/myproject",
			ProjectVersion: "3",
			Domain:         "example.com",
			Repository:     "github.com/example/op",
			ProjectName:    "myop",
			PluginChain:    []string{"go.kubebuilder.io/v4"},
			APIs: []project.APIInfo{
				{
					Group:      "apps",
					Version:    "v1alpha1",
					Kind:       "Foo",
					Controller: true,
					HasAPI:     true,
					Namespaced: true,
				},
			},
		}

		out, err := prompts.RenderProjectSummary("v4.0.0", ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring("example.com"))
		Expect(out).To(ContainSubstring("myop"))
		Expect(out).To(ContainSubstring("go.kubebuilder.io/v4"))
		Expect(out).To(ContainSubstring("Foo"))
	})
})
