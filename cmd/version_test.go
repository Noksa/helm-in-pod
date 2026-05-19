package cmd

import (
	"bytes"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("newVersionCmd", func() {
	It("registers the 'version' subcommand with a short description", func() {
		cmd := newVersionCmd()
		Expect(cmd.Use).To(Equal("version"))
		Expect(cmd.Short).NotTo(BeEmpty())
	})

	It("writes version, commit, build date, and go runtime to stdout", func() {
		cmd := newVersionCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		Expect(cmd.RunE(cmd, nil)).To(Succeed())

		got := out.String()
		Expect(got).To(ContainSubstring("helm-in-pod "))
		Expect(got).To(ContainSubstring("commit:"))
		Expect(got).To(ContainSubstring("built:"))
		Expect(got).To(ContainSubstring("go:"))
		Expect(got).To(ContainSubstring(runtime.Version()))
		Expect(got).To(HaveSuffix("\n"))
	})

	It("prints the dev defaults when ldflags are not set", func() {
		cmd := newVersionCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		Expect(cmd.RunE(cmd, nil)).To(Succeed())

		got := out.String()
		Expect(got).To(ContainSubstring("helm-in-pod dev"))
		Expect(got).To(ContainSubstring("commit: unknown"))
		Expect(got).To(ContainSubstring("built:  unknown"))
	})

	It("respects an overridden output stream", func() {
		origVersion, origCommit, origDate := version, commit, date
		DeferCleanup(func() {
			version, commit, date = origVersion, origCommit, origDate
		})
		version = "v1.2.3"
		commit = "deadbeef"
		date = "2026-05-19T00:00:00Z"

		cmd := newVersionCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		Expect(cmd.RunE(cmd, nil)).To(Succeed())

		got := out.String()
		Expect(got).To(ContainSubstring("helm-in-pod v1.2.3"))
		Expect(got).To(ContainSubstring("commit: deadbeef"))
		Expect(got).To(ContainSubstring("built:  2026-05-19T00:00:00Z"))
	})
})
