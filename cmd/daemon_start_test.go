package cmd

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

// daemonStartTestCmd is built once per Ordered Describe for efficiency.
var daemonStartTestCmd *cobra.Command

var _ = Describe("daemon start", Ordered, func() {
	BeforeAll(func() {
		daemonStartTestCmd = newDaemonStartCmd()
	})

	DescribeTable("registers expected flags",
		func(flagName string, shorthand string, expectedDefault string) {
			f := daemonStartTestCmd.Flags().Lookup(flagName)
			Expect(f).NotTo(BeNil(), "flag %s should be registered", flagName)
			if shorthand != "" {
				Expect(f.Shorthand).To(Equal(shorthand))
			}
			Expect(f.DefValue).To(Equal(expectedDefault))
		},
		Entry("name flag", "name", "", ""),
		Entry("force flag with shorthand", "force", "f", "false"),
		Entry("image flag", "image", "", "docker.io/noksa/kubectl-helm:v1.34.5-v4.1.1"),
		// add more as needed for coverage; these are representative
	)

	It("has correct command use and short description", func() {
		Expect(daemonStartTestCmd.Use).To(Equal("start"))
		Expect(daemonStartTestCmd.Short).To(ContainSubstring("persistent daemon pod"))
	})
})
