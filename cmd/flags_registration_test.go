package cmd

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/noksa/helm-in-pod/internal/cmdoptions"
)

var _ = Describe("Flag Registration", func() {
	Context("exec command flags", func() {
		var (
			execCmd *cobra.Command
			opts    *cmdoptions.ExecOptions
		)

		BeforeEach(func() {
			opts = &cmdoptions.ExecOptions{}
			execCmd = &cobra.Command{}
			addExecOptionsFlags(execCmd, opts)
		})

		It("should register all pod creation flags", func() {
			flags := []string{
				"run-as-user", "run-as-group",
				"labels", "annotations", "create-pdb",
				"cpu", "memory",
				"cpu-request", "cpu-limit", "memory-request", "memory-limit",
				"host-network", "tolerations", "node-selector",
				"image-pull-secret", "pull-policy", "image",
				"volume", "service-account", "dry-run",
			}
			for _, name := range flags {
				Expect(execCmd.Flags().Lookup(name)).NotTo(BeNil(), "flag --%s should be registered", name)
			}
		})

		It("should register all runtime flags", func() {
			flags := []string{
				"env", "subst-env", "copy-repo", "update-repo",
				"copy", "copy-attempts", "update-repo-attempts",
				"copy-from",
			}
			for _, name := range flags {
				Expect(execCmd.Flags().Lookup(name)).NotTo(BeNil(), "flag --%s should be registered", name)
			}
		})

		It("should have correct default for --run-as-user", func() {
			Expect(opts.RunAsUser).To(Equal(int64(-1)))
		})

		It("should have correct default for --run-as-group", func() {
			Expect(opts.RunAsGroup).To(Equal(int64(-1)))
		})

		It("should have correct default for --create-pdb", func() {
			Expect(opts.CreatePDB).To(BeTrue())
		})

		It("should have correct default for --pull-policy", func() {
			Expect(opts.PullPolicy).To(Equal("IfNotPresent"))
		})

		It("should have correct default for --copy-repo (exec)", func() {
			// addExecOptionsFlags calls addRuntimeFlags with copyRepoDefault=true
			Expect(opts.CopyRepo).To(BeTrue())
		})

		It("should have correct default for --copy-attempts", func() {
			Expect(opts.CopyAttempts).To(Equal(3))
		})

		It("should have correct default for --update-repo-attempts", func() {
			Expect(opts.UpdateRepoAttempts).To(Equal(3))
		})

		It("should have correct default for --host-network", func() {
			Expect(opts.HostNetwork).To(BeFalse())
		})

		It("should mark --cpu as deprecated", func() {
			f := execCmd.Flags().Lookup("cpu")
			Expect(f.Deprecated).NotTo(BeEmpty())
		})

		It("should mark --memory as deprecated", func() {
			f := execCmd.Flags().Lookup("memory")
			Expect(f.Deprecated).NotTo(BeEmpty())
		})

		It("should have shorthand -e for --env", func() {
			f := execCmd.Flags().Lookup("env")
			Expect(f.Shorthand).To(Equal("e"))
		})

		It("should have shorthand -s for --subst-env", func() {
			f := execCmd.Flags().Lookup("subst-env")
			Expect(f.Shorthand).To(Equal("s"))
		})

		It("should have shorthand -c for --copy", func() {
			f := execCmd.Flags().Lookup("copy")
			Expect(f.Shorthand).To(Equal("c"))
		})

		It("should have shorthand -i for --image", func() {
			f := execCmd.Flags().Lookup("image")
			Expect(f.Shorthand).To(Equal("i"))
		})
	})

	Context("daemon exec runtime flags", func() {
		It("should default --copy-repo to false for daemon exec", func() {
			opts := &cmdoptions.ExecOptions{}
			cmd := &cobra.Command{}
			addRuntimeFlags(cmd, opts, false)
			Expect(opts.CopyRepo).To(BeFalse())
		})
	})

	Context("flag value parsing", func() {
		var (
			testCmd *cobra.Command
			opts    *cmdoptions.ExecOptions
		)

		BeforeEach(func() {
			opts = &cmdoptions.ExecOptions{}
			testCmd = &cobra.Command{}
			addExecOptionsFlags(testCmd, opts)
		})

		It("should parse --run-as-user value", func() {
			Expect(testCmd.Flags().Set("run-as-user", "1000")).To(Succeed())
			Expect(opts.RunAsUser).To(Equal(int64(1000)))
		})

		It("should parse --run-as-group value", func() {
			Expect(testCmd.Flags().Set("run-as-group", "2000")).To(Succeed())
			Expect(opts.RunAsGroup).To(Equal(int64(2000)))
		})

		It("should parse --labels as key=value pairs", func() {
			Expect(testCmd.Flags().Set("labels", "app=test,env=dev")).To(Succeed())
			Expect(opts.Labels).To(HaveKeyWithValue("app", "test"))
			Expect(opts.Labels).To(HaveKeyWithValue("env", "dev"))
		})

		It("should parse --annotations as key=value pairs", func() {
			Expect(testCmd.Flags().Set("annotations", "note=hello")).To(Succeed())
			Expect(opts.Annotations).To(HaveKeyWithValue("note", "hello"))
		})

		It("should parse --env as key=value pairs", func() {
			Expect(testCmd.Flags().Set("env", "FOO=bar,BAZ=qux")).To(Succeed())
			Expect(opts.Env).To(HaveKeyWithValue("FOO", "bar"))
			Expect(opts.Env).To(HaveKeyWithValue("BAZ", "qux"))
		})

		It("should parse --subst-env as string slice", func() {
			Expect(testCmd.Flags().Set("subst-env", "HOME")).To(Succeed())
			Expect(testCmd.Flags().Set("subst-env", "PATH")).To(Succeed())
			Expect(opts.SubstEnv).To(ContainElements("HOME", "PATH"))
		})

		It("should parse --tolerations as string slice", func() {
			Expect(testCmd.Flags().Set("tolerations", "::Exists")).To(Succeed())
			Expect(opts.Tolerations).To(ContainElement("::Exists"))
		})

		It("should parse --node-selector as key=value pairs", func() {
			Expect(testCmd.Flags().Set("node-selector", "disktype=ssd")).To(Succeed())
			Expect(opts.NodeSelector).To(HaveKeyWithValue("disktype", "ssd"))
		})

		It("should parse --host-network as bool", func() {
			Expect(testCmd.Flags().Set("host-network", "true")).To(Succeed())
			Expect(opts.HostNetwork).To(BeTrue())
		})

		It("should parse --create-pdb as bool", func() {
			Expect(testCmd.Flags().Set("create-pdb", "false")).To(Succeed())
			Expect(opts.CreatePDB).To(BeFalse())
		})

		It("should parse --image value", func() {
			Expect(testCmd.Flags().Set("image", "my-image:v2")).To(Succeed())
			Expect(opts.Image).To(Equal("my-image:v2"))
		})

		It("should parse --image-pull-secret value", func() {
			Expect(testCmd.Flags().Set("image-pull-secret", "my-secret")).To(Succeed())
			Expect(opts.ImagePullSecret).To(Equal("my-secret"))
		})

		It("should parse --pull-policy value", func() {
			Expect(testCmd.Flags().Set("pull-policy", "Always")).To(Succeed())
			Expect(opts.PullPolicy).To(Equal("Always"))
		})

		It("should parse --copy as string slice", func() {
			Expect(testCmd.Flags().Set("copy", "/host/path:/pod/path")).To(Succeed())
			Expect(opts.Files).To(ContainElement("/host/path:/pod/path"))
		})

		It("should parse --copy-attempts value", func() {
			Expect(testCmd.Flags().Set("copy-attempts", "5")).To(Succeed())
			Expect(opts.CopyAttempts).To(Equal(5))
		})

		It("should parse --update-repo-attempts value", func() {
			Expect(testCmd.Flags().Set("update-repo-attempts", "10")).To(Succeed())
			Expect(opts.UpdateRepoAttempts).To(Equal(10))
		})

		It("should parse --volume as string slice", func() {
			Expect(testCmd.Flags().Set("volume", "pvc:my-claim:/data")).To(Succeed())
			Expect(opts.Volumes).To(ContainElement("pvc:my-claim:/data"))
		})

		It("should parse multiple --volume flags", func() {
			Expect(testCmd.Flags().Set("volume", "pvc:claim1:/data")).To(Succeed())
			Expect(testCmd.Flags().Set("volume", "secret:sec1:/creds")).To(Succeed())
			Expect(opts.Volumes).To(ContainElements("pvc:claim1:/data", "secret:sec1:/creds"))
		})

		It("should parse --service-account value", func() {
			Expect(testCmd.Flags().Set("service-account", "my-sa")).To(Succeed())
			Expect(opts.ServiceAccount).To(Equal("my-sa"))
		})

		It("should have empty default for --service-account", func() {
			Expect(opts.ServiceAccount).To(BeEmpty())
		})

		It("should parse --dry-run as bool", func() {
			Expect(testCmd.Flags().Set("dry-run", "true")).To(Succeed())
			Expect(opts.DryRun).To(BeTrue())
		})

		It("should have false default for --dry-run", func() {
			Expect(opts.DryRun).To(BeFalse())
		})

		It("should parse --copy-from as string slice", func() {
			Expect(testCmd.Flags().Set("copy-from", "/pod/path:/host/path")).To(Succeed())
			Expect(opts.CopyFrom).To(ContainElement("/pod/path:/host/path"))
		})

		It("should parse multiple --copy-from flags", func() {
			Expect(testCmd.Flags().Set("copy-from", "/tmp/a:./a")).To(Succeed())
			Expect(testCmd.Flags().Set("copy-from", "/tmp/b:./b")).To(Succeed())
			Expect(opts.CopyFrom).To(ContainElements("/tmp/a:./a", "/tmp/b:./b"))
		})
	})
})
