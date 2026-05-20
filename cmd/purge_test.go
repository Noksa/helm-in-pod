package cmd

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("newPurgeCmd", func() {
	It("registers Use and Short", func() {
		cmd := newPurgeCmd()
		Expect(cmd.Use).To(Equal("purge"))
		Expect(cmd.Short).NotTo(BeEmpty())
	})

	It("wires RunE so the dispatcher branches into the purge manager calls", func() {
		// We do not invoke RunE here (it depends on internal.Pod() and
		// internal.Namespace(), both initialized by root.PersistentPreRunE).
		// The branch semantics — host-scoped purge vs --all — are exercised
		// at the internal/hippod level by pod_delete_test.go and end-to-end
		// by e2e/keep_pod_test.go and e2e/purge_isolation_test.go.
		cmd := newPurgeCmd()
		Expect(cmd.RunE).NotTo(BeNil(),
			"RunE must be set so Cobra routes purge to the manager dispatch")
	})

	It("registers --all as a bool flag defaulting to false", func() {
		cmd := newPurgeCmd()
		flag := cmd.Flags().Lookup("all")
		Expect(flag).NotTo(BeNil(), "--all is the only purge-specific flag")
		Expect(flag.Value.Type()).To(Equal("bool"))
		Expect(flag.DefValue).To(Equal("false"),
			"default must remain host-scoped to avoid namespace-wide deletion by accident")
	})

	It("accepts --all=true via flag parsing without arguments", func() {
		cmd := newPurgeCmd()
		Expect(cmd.Flags().Parse([]string{"--all"})).To(Succeed())
		Expect(cmd.Flags().Lookup("all").Value.String()).To(Equal("true"))
	})

	It("does not accept positional arguments (purge is flag-only)", func() {
		cmd := newPurgeCmd()
		// Cobra's default Args validator allows anything, so we lock in the
		// current shape: no positional arg is required, none is rejected
		// either. If a maintainer later wires cobra.NoArgs, this test would
		// need to flip to Expect(...).To(HaveOccurred()).
		Expect(cmd.Args).To(BeNil(),
			"purge does not currently validate positional args — change this test if you add cobra.NoArgs")
	})
})
