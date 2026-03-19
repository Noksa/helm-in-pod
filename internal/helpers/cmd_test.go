package helpers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

var _ = Describe("IsCompletionCmd", func() {
	It("should return true for a command named 'completion'", func() {
		cmd := &cobra.Command{Use: "completion"}
		Expect(IsCompletionCmd(cmd)).To(BeTrue())
	})

	It("should return true for a child of 'completion'", func() {
		parent := &cobra.Command{Use: "completion"}
		child := &cobra.Command{Use: "bash"}
		parent.AddCommand(child)
		Expect(IsCompletionCmd(child)).To(BeTrue())
	})

	It("should return true for a grandchild of 'completion'", func() {
		root := &cobra.Command{Use: "completion"}
		mid := &cobra.Command{Use: "bash"}
		leaf := &cobra.Command{Use: "generate"}
		root.AddCommand(mid)
		mid.AddCommand(leaf)
		Expect(IsCompletionCmd(leaf)).To(BeTrue())
	})

	It("should return false for a non-completion command", func() {
		cmd := &cobra.Command{Use: "exec"}
		Expect(IsCompletionCmd(cmd)).To(BeFalse())
	})

	It("should return false for a child of a non-completion command", func() {
		parent := &cobra.Command{Use: "daemon"}
		child := &cobra.Command{Use: "start"}
		parent.AddCommand(child)
		Expect(IsCompletionCmd(child)).To(BeFalse())
	})

	It("should return false for a root command with no parent", func() {
		cmd := &cobra.Command{Use: "in-pod"}
		Expect(IsCompletionCmd(cmd)).To(BeFalse())
	})
})
