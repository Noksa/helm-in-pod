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

	Context("daemon start command flags", func() {
		var startCmd *cobra.Command

		BeforeEach(func() {
			startCmd = newDaemonStartCmd()
		})

		It("should register daemon-specific flags", func() {
			Expect(startCmd.Flags().Lookup("name")).NotTo(BeNil())
			Expect(startCmd.Flags().Lookup("force")).NotTo(BeNil())
		})

		It("should inherit pod creation flags", func() {
			flags := []string{
				"run-as-user", "run-as-group",
				"labels", "annotations", "create-pdb",
				"cpu", "memory",
				"cpu-request", "cpu-limit", "memory-request", "memory-limit",
				"host-network", "tolerations", "node-selector",
				"image-pull-secret", "pull-policy", "image",
				"volume", "service-account", "dry-run",
				"active-deadline-seconds",
			}
			for _, name := range flags {
				Expect(startCmd.Flags().Lookup(name)).NotTo(BeNil(), "flag --%s should be registered", name)
			}
		})

		It("should inherit runtime flags", func() {
			flags := []string{
				"env", "subst-env", "copy-repo", "update-repo",
				"copy", "copy-attempts", "update-repo-attempts",
				"copy-from",
			}
			for _, name := range flags {
				Expect(startCmd.Flags().Lookup(name)).NotTo(BeNil(), "flag --%s should be registered", name)
			}
		})

		It("should have empty default for --name", func() {
			Expect(startCmd.Flags().Lookup("name").DefValue).To(Equal(""))
		})

		It("should have false default for --force", func() {
			Expect(startCmd.Flags().Lookup("force").DefValue).To(Equal("false"))
		})

		It("should have shorthand -f for --force", func() {
			Expect(startCmd.Flags().Lookup("force").Shorthand).To(Equal("f"))
		})

		It("should default --copy-repo to true (inherits exec defaults)", func() {
			Expect(startCmd.Flags().Lookup("copy-repo").DefValue).To(Equal("true"))
		})

		It("should parse --name value", func() {
			Expect(startCmd.Flags().Set("name", "my-daemon")).To(Succeed())
			Expect(startCmd.Flags().Lookup("name").Value.String()).To(Equal("my-daemon"))
		})

		It("should parse --force value", func() {
			Expect(startCmd.Flags().Set("force", "true")).To(Succeed())
			Expect(startCmd.Flags().Lookup("force").Value.String()).To(Equal("true"))
		})

		It("should not register daemon-exec-only flags", func() {
			Expect(startCmd.Flags().Lookup("update-all-repos")).To(BeNil())
			Expect(startCmd.Flags().Lookup("clean")).To(BeNil())
			Expect(startCmd.Flags().Lookup("shell")).To(BeNil())
		})
	})

	Context("daemon exec command flags", func() {
		var execCmd *cobra.Command

		BeforeEach(func() {
			execCmd = newDaemonExecCmd()
		})

		It("should register daemon-exec-specific flags", func() {
			Expect(execCmd.Flags().Lookup("name")).NotTo(BeNil())
			Expect(execCmd.Flags().Lookup("update-all-repos")).NotTo(BeNil())
			Expect(execCmd.Flags().Lookup("clean")).NotTo(BeNil())
		})

		It("should inherit runtime flags", func() {
			flags := []string{
				"env", "subst-env", "copy-repo", "update-repo",
				"copy", "copy-attempts", "update-repo-attempts",
				"copy-from",
			}
			for _, name := range flags {
				Expect(execCmd.Flags().Lookup(name)).NotTo(BeNil(), "flag --%s should be registered", name)
			}
		})

		It("should default --copy-repo to false for daemon exec", func() {
			Expect(execCmd.Flags().Lookup("copy-repo").DefValue).To(Equal("false"))
		})

		It("should have false default for --update-all-repos", func() {
			Expect(execCmd.Flags().Lookup("update-all-repos").DefValue).To(Equal("false"))
		})

		It("should have empty default for --clean", func() {
			Expect(execCmd.Flags().Lookup("clean").DefValue).To(Equal("[]"))
		})

		It("should parse --update-all-repos value", func() {
			Expect(execCmd.Flags().Set("update-all-repos", "true")).To(Succeed())
			Expect(execCmd.Flags().Lookup("update-all-repos").Value.String()).To(Equal("true"))
		})

		It("should parse --clean as string slice", func() {
			Expect(execCmd.Flags().Set("clean", "/tmp/a")).To(Succeed())
			Expect(execCmd.Flags().Set("clean", "/tmp/b")).To(Succeed())
			Expect(execCmd.Flags().Lookup("clean").Value.String()).To(Equal("[/tmp/a,/tmp/b]"))
		})

		It("should not register pod creation flags", func() {
			// daemon exec uses an existing daemon pod, so pod creation flags do not apply
			Expect(execCmd.Flags().Lookup("image")).To(BeNil())
			Expect(execCmd.Flags().Lookup("cpu-request")).To(BeNil())
			Expect(execCmd.Flags().Lookup("volume")).To(BeNil())
			Expect(execCmd.Flags().Lookup("service-account")).To(BeNil())
			Expect(execCmd.Flags().Lookup("dry-run")).To(BeNil())
		})
	})

	Context("daemon shell command flags", func() {
		var shellCmd *cobra.Command

		BeforeEach(func() {
			shellCmd = newDaemonShellCmd()
		})

		It("should register --name and --shell", func() {
			Expect(shellCmd.Flags().Lookup("name")).NotTo(BeNil())
			Expect(shellCmd.Flags().Lookup("shell")).NotTo(BeNil())
		})

		It("should default --shell to sh", func() {
			Expect(shellCmd.Flags().Lookup("shell").DefValue).To(Equal("sh"))
		})

		It("should parse --shell value", func() {
			Expect(shellCmd.Flags().Set("shell", "bash")).To(Succeed())
			Expect(shellCmd.Flags().Lookup("shell").Value.String()).To(Equal("bash"))
		})
	})

	Context("daemon stop command flags", func() {
		It("should register only --name", func() {
			stopCmd := newDaemonStopCmd()
			Expect(stopCmd.Flags().Lookup("name")).NotTo(BeNil())
			Expect(stopCmd.Flags().Lookup("force")).To(BeNil())
		})
	})

	Context("daemon status command flags", func() {
		It("should register only --name", func() {
			statusCmd := newDaemonStatusCmd()
			Expect(statusCmd.Flags().Lookup("name")).NotTo(BeNil())
		})
	})

	Context("daemon list command flags", func() {
		It("should register no flags and expose ls alias", func() {
			listCmd := newDaemonListCmd()
			Expect(listCmd.Aliases).To(ContainElement("ls"))
			// list takes no flags of its own
			Expect(listCmd.Flags().Lookup("name")).To(BeNil())
		})
	})

	Context("purge command flags", func() {
		var purgeCmd *cobra.Command

		BeforeEach(func() {
			purgeCmd = newPurgeCmd()
		})

		It("should register --all", func() {
			Expect(purgeCmd.Flags().Lookup("all")).NotTo(BeNil())
		})

		It("should default --all to false", func() {
			Expect(purgeCmd.Flags().Lookup("all").DefValue).To(Equal("false"))
		})

		It("should parse --all value", func() {
			Expect(purgeCmd.Flags().Set("all", "true")).To(Succeed())
			Expect(purgeCmd.Flags().Lookup("all").Value.String()).To(Equal("true"))
		})
	})

	Context("root command persistent flags", func() {
		var rootCmd *cobra.Command

		BeforeEach(func() {
			rootCmd = newRootCmd()
		})

		It("should register --verbose-logs and --timeout as persistent flags", func() {
			Expect(rootCmd.PersistentFlags().Lookup("verbose-logs")).NotTo(BeNil())
			Expect(rootCmd.PersistentFlags().Lookup("timeout")).NotTo(BeNil())
		})

		It("should default --verbose-logs to false", func() {
			Expect(rootCmd.PersistentFlags().Lookup("verbose-logs").DefValue).To(Equal("false"))
		})

		It("should default --timeout to 0s", func() {
			Expect(rootCmd.PersistentFlags().Lookup("timeout").DefValue).To(Equal("0s"))
		})

		It("should parse --verbose-logs value", func() {
			Expect(rootCmd.PersistentFlags().Set("verbose-logs", "true")).To(Succeed())
			Expect(rootCmd.PersistentFlags().Lookup("verbose-logs").Value.String()).To(Equal("true"))
		})

		It("should parse --timeout as a duration", func() {
			Expect(rootCmd.PersistentFlags().Set("timeout", "30m")).To(Succeed())
			Expect(rootCmd.PersistentFlags().Lookup("timeout").Value.String()).To(Equal("30m0s"))
		})
	})
})
