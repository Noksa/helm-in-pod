package cmdoptions_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/noksa/helm-in-pod/internal/cmdoptions"
)

var _ = Describe("ExecOptions.ParseFileMappings", func() {
	It("does nothing when Files is nil", func() {
		opts := cmdoptions.ExecOptions{}
		opts.ParseFileMappings()
		Expect(opts.FilesAsMap).To(BeNil())
	})

	It("does nothing when Files is empty", func() {
		opts := cmdoptions.ExecOptions{Files: []string{}}
		opts.ParseFileMappings()
		Expect(opts.FilesAsMap).To(BeNil())
	})

	It("parses a single host:pod mapping", func() {
		opts := cmdoptions.ExecOptions{Files: []string{"/host/values.yaml:/pod/values.yaml"}}
		opts.ParseFileMappings()
		Expect(opts.FilesAsMap).To(Equal(map[string]string{
			"/host/values.yaml": "/pod/values.yaml",
		}))
	})

	It("parses comma-separated mappings within a single Files entry", func() {
		opts := cmdoptions.ExecOptions{Files: []string{"/host/a:/pod/a,/host/b:/pod/b"}}
		opts.ParseFileMappings()
		Expect(opts.FilesAsMap).To(Equal(map[string]string{
			"/host/a": "/pod/a",
			"/host/b": "/pod/b",
		}))
	})

	It("parses multiple Files entries (repeated --copy flags)", func() {
		opts := cmdoptions.ExecOptions{Files: []string{
			"/host/a:/pod/a",
			"/host/b:/pod/b",
			"/host/c:/pod/c",
		}}
		opts.ParseFileMappings()
		Expect(opts.FilesAsMap).To(Equal(map[string]string{
			"/host/a": "/pod/a",
			"/host/b": "/pod/b",
			"/host/c": "/pod/c",
		}))
	})

	It("uses only the first colon as separator (preserves colons in pod path)", func() {
		opts := cmdoptions.ExecOptions{Files: []string{"/host/file:/pod/dir:subpath"}}
		opts.ParseFileMappings()
		Expect(opts.FilesAsMap).To(HaveKeyWithValue("/host/file", "/pod/dir:subpath"))
	})

	It("last duplicate source key wins", func() {
		opts := cmdoptions.ExecOptions{Files: []string{
			"/host/a:/pod/first",
			"/host/a:/pod/second",
		}}
		opts.ParseFileMappings()
		Expect(opts.FilesAsMap).To(Equal(map[string]string{
			"/host/a": "/pod/second",
		}))
	})

	It("locks in current behavior: silently drops entries with no colon (no error returned)", func() {
		opts := cmdoptions.ExecOptions{Files: []string{"missing-colon"}}
		opts.ParseFileMappings()
		Expect(opts.FilesAsMap).To(BeEmpty())
	})

	It("locks in current behavior: silently drops malformed segments in comma list", func() {
		opts := cmdoptions.ExecOptions{Files: []string{"/host/a:/pod/a,bogus,/host/b:/pod/b"}}
		opts.ParseFileMappings()
		Expect(opts.FilesAsMap).To(Equal(map[string]string{
			"/host/a": "/pod/a",
			"/host/b": "/pod/b",
		}))
	})

	It("allows empty source path when entry starts with colon", func() {
		opts := cmdoptions.ExecOptions{Files: []string{":/pod/path"}}
		opts.ParseFileMappings()
		Expect(opts.FilesAsMap).To(HaveKeyWithValue("", "/pod/path"))
	})

	It("allows empty dest path (entry ending in colon)", func() {
		opts := cmdoptions.ExecOptions{Files: []string{"/host/path:"}}
		opts.ParseFileMappings()
		Expect(opts.FilesAsMap).To(HaveKeyWithValue("/host/path", ""))
	})

	It("handles trailing comma by silently dropping the empty segment", func() {
		opts := cmdoptions.ExecOptions{Files: []string{"/host/a:/pod/a,"}}
		opts.ParseFileMappings()
		Expect(opts.FilesAsMap).To(Equal(map[string]string{"/host/a": "/pod/a"}))
	})

	It("handles leading comma by silently dropping the empty segment", func() {
		opts := cmdoptions.ExecOptions{Files: []string{",/host/a:/pod/a"}}
		opts.ParseFileMappings()
		Expect(opts.FilesAsMap).To(Equal(map[string]string{"/host/a": "/pod/a"}))
	})

	It("handles consecutive commas by silently dropping empty segments", func() {
		opts := cmdoptions.ExecOptions{Files: []string{"/host/a:/pod/a,,/host/b:/pod/b"}}
		opts.ParseFileMappings()
		Expect(opts.FilesAsMap).To(Equal(map[string]string{
			"/host/a": "/pod/a",
			"/host/b": "/pod/b",
		}))
	})
})
