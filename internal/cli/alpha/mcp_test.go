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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("NewMCPCommand", func() {
	It("returns a non-nil command with expected metadata", func() {
		cmd := NewMCPCommand()
		Expect(cmd).NotTo(BeNil())
		Expect(cmd.Use).To(Equal("mcp"))
		Expect(cmd.Short).To(ContainSubstring("MCP"))
		Expect(cmd.Long).To(ContainSubstring("EXPERIMENTAL"))
		Expect(cmd.Long).To(ContainSubstring("kubebuilder://version"))
		Expect(cmd.Long).To(ContainSubstring("kubebuilder://project/config"))
	})

	It("defines --transport flag defaulting to stdio", func() {
		cmd := NewMCPCommand()
		f := cmd.Flags().Lookup("transport")
		Expect(f).NotTo(BeNil())
		Expect(f.DefValue).To(Equal("stdio"))
	})

	It("defines --project-dir flag defaulting to empty string", func() {
		cmd := NewMCPCommand()
		f := cmd.Flags().Lookup("project-dir")
		Expect(f).NotTo(BeNil())
		Expect(f.DefValue).To(Equal(""))
	})

	It("returns an error for an unsupported transport", func() {
		cmd := NewMCPCommand()
		Expect(cmd.Flags().Set("transport", "http")).To(Succeed())
		err := cmd.RunE(cmd, []string{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unsupported transport"))
	})

	It("includes alpha mcp in the example text", func() {
		cmd := NewMCPCommand()
		Expect(cmd.Example).To(ContainSubstring("kubebuilder alpha mcp"))
	})
})
