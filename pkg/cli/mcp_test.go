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

package cli

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("mcp", func() {
	Context("newMCPCmd", func() {
		It("should create a command with the correct use and short description", func() {
			cli := CLI{commandName: "kubebuilder", cliVersion: "v4.0.0"}
			cmd := cli.newMCPCmd()

			Expect(cmd.Use).To(Equal("mcp"))
			Expect(cmd.Short).To(ContainSubstring("MCP"))
			Expect(cmd.Long).To(ContainSubstring("EXPERIMENTAL"))
			Expect(cmd.Long).To(ContainSubstring("kubebuilder://version"))
			Expect(cmd.Long).To(ContainSubstring("kubebuilder://project/config"))
		})

		It("should define --transport and --project-dir flags", func() {
			cli := CLI{commandName: "kubebuilder", cliVersion: "v4.0.0"}
			cmd := cli.newMCPCmd()

			transportFlag := cmd.Flags().Lookup("transport")
			Expect(transportFlag).NotTo(BeNil())
			Expect(transportFlag.DefValue).To(Equal("stdio"))

			projectDirFlag := cmd.Flags().Lookup("project-dir")
			Expect(projectDirFlag).NotTo(BeNil())
			Expect(projectDirFlag.DefValue).To(Equal(""))
		})

		It("should return an error for unsupported transports", func() {
			cli := CLI{commandName: "kubebuilder", cliVersion: "v4.0.0"}
			cmd := cli.newMCPCmd()

			// Set an unsupported transport flag value directly.
			Expect(cmd.Flags().Set("transport", "http")).To(Succeed())
			err := cmd.RunE(cmd, []string{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unsupported transport"))
		})

		It("should include the command name in examples", func() {
			cli := CLI{commandName: "kb", cliVersion: "v4.0.0"}
			cmd := cli.newMCPCmd()

			Expect(cmd.Example).To(ContainSubstring("kb mcp"))
		})
	})
})
