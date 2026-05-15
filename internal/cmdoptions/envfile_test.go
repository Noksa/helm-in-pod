package cmdoptions_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/noksa/helm-in-pod/internal/cmdoptions"
)

var _ = Describe("ParseEnvFiles", func() {
	var tmpDir string

	BeforeEach(func() {
		tmpDir = GinkgoT().TempDir()
	})

	writeFile := func(name, content string) string {
		p := filepath.Join(tmpDir, name)
		Expect(os.WriteFile(p, []byte(content), 0644)).To(Succeed())
		return p
	}

	It("parses simple KEY=VALUE pairs", func() {
		path := writeFile("env", "FOO=bar\nBAZ=qux\n")
		opts := cmdoptions.ExecOptions{EnvFiles: []string{path}}
		Expect(opts.ParseEnvFiles()).To(Succeed())
		Expect(opts.Env).To(Equal(map[string]string{"FOO": "bar", "BAZ": "qux"}))
	})

	It("skips comments and empty lines", func() {
		path := writeFile("env", "# comment\n\nKEY=value\n  # indented comment\n")
		opts := cmdoptions.ExecOptions{EnvFiles: []string{path}}
		Expect(opts.ParseEnvFiles()).To(Succeed())
		Expect(opts.Env).To(Equal(map[string]string{"KEY": "value"}))
	})

	It("handles double-quoted values", func() {
		path := writeFile("env", `DB_URL="postgres://user:pass@host/db"`)
		opts := cmdoptions.ExecOptions{EnvFiles: []string{path}}
		Expect(opts.ParseEnvFiles()).To(Succeed())
		Expect(opts.Env["DB_URL"]).To(Equal("postgres://user:pass@host/db"))
	})

	It("handles single-quoted values", func() {
		path := writeFile("env", "TOKEN='abc123'")
		opts := cmdoptions.ExecOptions{EnvFiles: []string{path}}
		Expect(opts.ParseEnvFiles()).To(Succeed())
		Expect(opts.Env["TOKEN"]).To(Equal("abc123"))
	})

	It("handles values with equals signs", func() {
		path := writeFile("env", "CONN=postgres://host?opt=val&x=y")
		opts := cmdoptions.ExecOptions{EnvFiles: []string{path}}
		Expect(opts.ParseEnvFiles()).To(Succeed())
		Expect(opts.Env["CONN"]).To(Equal("postgres://host?opt=val&x=y"))
	})

	It("handles export prefix", func() {
		path := writeFile("env", "export MY_VAR=hello\nexport OTHER=world")
		opts := cmdoptions.ExecOptions{EnvFiles: []string{path}}
		Expect(opts.ParseEnvFiles()).To(Succeed())
		Expect(opts.Env).To(Equal(map[string]string{"MY_VAR": "hello", "OTHER": "world"}))
	})

	It("explicit --env flags take precedence over env file", func() {
		path := writeFile("env", "KEY=from-file\nOTHER=file-only")
		opts := cmdoptions.ExecOptions{
			Env:      map[string]string{"KEY": "from-flag"},
			EnvFiles: []string{path},
		}
		Expect(opts.ParseEnvFiles()).To(Succeed())
		Expect(opts.Env["KEY"]).To(Equal("from-flag"))
		Expect(opts.Env["OTHER"]).To(Equal("file-only"))
	})

	It("later files override earlier files", func() {
		path1 := writeFile("env1", "A=first\nB=first")
		path2 := writeFile("env2", "A=second")
		opts := cmdoptions.ExecOptions{EnvFiles: []string{path1, path2}}
		Expect(opts.ParseEnvFiles()).To(Succeed())
		Expect(opts.Env["A"]).To(Equal("second"))
		Expect(opts.Env["B"]).To(Equal("first"))
	})

	It("returns error for missing file", func() {
		opts := cmdoptions.ExecOptions{EnvFiles: []string{"/nonexistent/.env"}}
		err := opts.ParseEnvFiles()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to open env file"))
	})

	It("returns error for invalid format (no equals)", func() {
		path := writeFile("env", "INVALID_LINE")
		opts := cmdoptions.ExecOptions{EnvFiles: []string{path}}
		err := opts.ParseEnvFiles()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("line 1"))
	})

	It("handles empty value", func() {
		path := writeFile("env", "EMPTY=")
		opts := cmdoptions.ExecOptions{EnvFiles: []string{path}}
		Expect(opts.ParseEnvFiles()).To(Succeed())
		Expect(opts.Env["EMPTY"]).To(Equal(""))
	})

	It("does nothing when no env files specified", func() {
		opts := cmdoptions.ExecOptions{}
		Expect(opts.ParseEnvFiles()).To(Succeed())
		Expect(opts.Env).To(BeNil())
	})
})
