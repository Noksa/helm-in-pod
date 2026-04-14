package hippod

import (
	"time"

	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("buildPodSpec", func() {
	var baseOpts func() cmdoptions.ExecOptions

	BeforeEach(func() {
		baseOpts = func() cmdoptions.ExecOptions {
			return cmdoptions.ExecOptions{
				Image:         "docker.io/noksa/kubectl-helm:latest",
				PullPolicy:    "IfNotPresent",
				CpuRequest:    "500m",
				CpuLimit:      "1000m",
				MemoryRequest: "256Mi",
				MemoryLimit:   "512Mi",
				RunAsUser:     -1,
				RunAsGroup:    -1,
				Timeout:       10 * time.Minute,
			}
		}
	})

	Context("environment variables", func() {
		It("should set explicit env vars from --env flag", func() {
			opts := baseOpts()
			opts.Env = map[string]string{
				"FOO": "bar",
				"BAZ": "qux",
			}
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			envNames := envVarNames(spec.Containers[0].Env)
			Expect(envNames).To(ContainElements("FOO", "BAZ", "TIMEOUT"))
			Expect(findEnvVar(spec.Containers[0].Env, "FOO")).To(Equal("bar"))
			Expect(findEnvVar(spec.Containers[0].Env, "BAZ")).To(Equal("qux"))
		})

		It("should substitute env vars from --subst-env using host environment", func() {
			opts := baseOpts()
			opts.SubstEnv = []string{"PATH"}
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			envNames := envVarNames(spec.Containers[0].Env)
			Expect(envNames).To(ContainElement("PATH"))
		})

		It("should always include TIMEOUT env var", func() {
			opts := baseOpts()
			opts.Timeout = 30 * time.Minute
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(findEnvVar(spec.Containers[0].Env, "TIMEOUT")).To(Equal("1800"))
		})

		It("should handle empty env maps", func() {
			opts := baseOpts()
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			// Only TIMEOUT should be present
			Expect(spec.Containers[0].Env).To(HaveLen(1))
			Expect(spec.Containers[0].Env[0].Name).To(Equal("TIMEOUT"))
		})

		It("should include both --env and --subst-env vars together", func() {
			opts := baseOpts()
			opts.Env = map[string]string{"MY_VAR": "hello"}
			opts.SubstEnv = []string{"HOME"}
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			envNames := envVarNames(spec.Containers[0].Env)
			Expect(envNames).To(ContainElements("MY_VAR", "HOME", "TIMEOUT"))
		})
	})

	Context("resource requests and limits", func() {
		It("should set CPU and memory requests and limits", func() {
			opts := baseOpts()
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			res := spec.Containers[0].Resources
			Expect(res.Requests.Cpu().String()).To(Equal("500m"))
			Expect(res.Limits.Cpu().String()).To(Equal("1"))
			Expect(res.Requests.Memory().String()).To(Equal("256Mi"))
			Expect(res.Limits.Memory().String()).To(Equal("512Mi"))
		})

		It("should handle request-only without limit", func() {
			opts := baseOpts()
			opts.CpuLimit = ""
			opts.MemoryLimit = ""
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			res := spec.Containers[0].Resources
			Expect(res.Requests.Cpu().String()).To(Equal("500m"))
			_, hasCpuLimit := res.Limits["cpu"]
			Expect(hasCpuLimit).To(BeFalse())
			Expect(res.Requests.Memory().String()).To(Equal("256Mi"))
			_, hasMemLimit := res.Limits["memory"]
			Expect(hasMemLimit).To(BeFalse())
		})

		It("should handle limit-only without request", func() {
			opts := baseOpts()
			opts.CpuRequest = ""
			opts.MemoryRequest = ""
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			res := spec.Containers[0].Resources
			_, hasCpuReq := res.Requests["cpu"]
			Expect(hasCpuReq).To(BeFalse())
			Expect(res.Limits.Cpu().String()).To(Equal("1"))
			_, hasMemReq := res.Requests["memory"]
			Expect(hasMemReq).To(BeFalse())
			Expect(res.Limits.Memory().String()).To(Equal("512Mi"))
		})

		It("should skip resources entirely when all are empty", func() {
			opts := baseOpts()
			opts.CpuRequest = ""
			opts.CpuLimit = ""
			opts.MemoryRequest = ""
			opts.MemoryLimit = ""
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			res := spec.Containers[0].Resources
			Expect(res.Requests).To(BeEmpty())
			Expect(res.Limits).To(BeEmpty())
		})

		It("should skip resources with value '0'", func() {
			opts := baseOpts()
			opts.CpuRequest = "0"
			opts.CpuLimit = "0"
			opts.MemoryRequest = "0"
			opts.MemoryLimit = "0"
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			res := spec.Containers[0].Resources
			Expect(res.Requests).To(BeEmpty())
			Expect(res.Limits).To(BeEmpty())
		})

		It("should set request but skip limit when limit is '0'", func() {
			opts := baseOpts()
			opts.CpuRequest = "50m"
			opts.CpuLimit = "0"
			opts.MemoryRequest = "64Mi"
			opts.MemoryLimit = "0"
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			res := spec.Containers[0].Resources
			Expect(res.Requests.Cpu().String()).To(Equal("50m"))
			_, hasCpuLimit := res.Limits["cpu"]
			Expect(hasCpuLimit).To(BeFalse(), "cpu limit should be absent when set to '0'")
			Expect(res.Requests.Memory().String()).To(Equal("64Mi"))
			_, hasMemLimit := res.Limits["memory"]
			Expect(hasMemLimit).To(BeFalse(), "memory limit should be absent when set to '0'")
		})

		It("should set limit but skip request when request is '0'", func() {
			opts := baseOpts()
			opts.CpuRequest = "0"
			opts.CpuLimit = "500m"
			opts.MemoryRequest = "0"
			opts.MemoryLimit = "256Mi"
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			res := spec.Containers[0].Resources
			_, hasCpuReq := res.Requests["cpu"]
			Expect(hasCpuReq).To(BeFalse(), "cpu request should be absent when set to '0'")
			Expect(res.Limits.Cpu().String()).To(Equal("500m"))
			_, hasMemReq := res.Requests["memory"]
			Expect(hasMemReq).To(BeFalse(), "memory request should be absent when set to '0'")
			Expect(res.Limits.Memory().String()).To(Equal("256Mi"))
		})

		It("should parse various resource formats", func() {
			opts := baseOpts()
			opts.CpuRequest = "2"
			opts.CpuLimit = "4000m"
			opts.MemoryRequest = "1Gi"
			opts.MemoryLimit = "2Gi"
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			res := spec.Containers[0].Resources
			Expect(res.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("2")))
			Expect(res.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse("4000m")))
			Expect(res.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse("1Gi")))
			Expect(res.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse("2Gi")))
		})

		It("should return an error for invalid --cpu-request instead of panicking", func() {
			opts := baseOpts()
			opts.CpuRequest = "badvalue"
			_, err := buildPodSpec(opts, false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("--cpu-request"))
			Expect(err.Error()).To(ContainSubstring("badvalue"))
		})

		It("should return an error for invalid --cpu-limit instead of panicking", func() {
			opts := baseOpts()
			opts.CpuLimit = "not-a-quantity"
			_, err := buildPodSpec(opts, false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("--cpu-limit"))
			Expect(err.Error()).To(ContainSubstring("not-a-quantity"))
		})

		It("should return an error for invalid --memory-request instead of panicking", func() {
			opts := baseOpts()
			opts.MemoryRequest = "5ZZZ"
			_, err := buildPodSpec(opts, false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("--memory-request"))
			Expect(err.Error()).To(ContainSubstring("5ZZZ"))
		})

		It("should return an error for invalid --memory-limit instead of panicking", func() {
			opts := baseOpts()
			opts.MemoryLimit = "xyz"
			_, err := buildPodSpec(opts, false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("--memory-limit"))
			Expect(err.Error()).To(ContainSubstring("xyz"))
		})
	})

	Context("security context", func() {
		It("should set RunAsUser when specified", func() {
			opts := baseOpts()
			opts.RunAsUser = 1000
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			sc := spec.Containers[0].SecurityContext
			Expect(sc.RunAsUser).NotTo(BeNil())
			Expect(*sc.RunAsUser).To(Equal(int64(1000)))
		})

		It("should set RunAsGroup when specified", func() {
			opts := baseOpts()
			opts.RunAsGroup = 2000
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			sc := spec.Containers[0].SecurityContext
			Expect(sc.RunAsGroup).NotTo(BeNil())
			Expect(*sc.RunAsGroup).To(Equal(int64(2000)))
		})

		It("should set both RunAsUser and RunAsGroup", func() {
			opts := baseOpts()
			opts.RunAsUser = 1000
			opts.RunAsGroup = 1000
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			sc := spec.Containers[0].SecurityContext
			Expect(*sc.RunAsUser).To(Equal(int64(1000)))
			Expect(*sc.RunAsGroup).To(Equal(int64(1000)))
		})

		It("should not set RunAsUser when -1 (default)", func() {
			opts := baseOpts()
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			sc := spec.Containers[0].SecurityContext
			Expect(sc.RunAsUser).To(BeNil())
		})

		It("should not set RunAsGroup when -1 (default)", func() {
			opts := baseOpts()
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			sc := spec.Containers[0].SecurityContext
			Expect(sc.RunAsGroup).To(BeNil())
		})

		It("should handle RunAsUser=0 (root)", func() {
			opts := baseOpts()
			opts.RunAsUser = 0
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			sc := spec.Containers[0].SecurityContext
			Expect(sc.RunAsUser).NotTo(BeNil())
			Expect(*sc.RunAsUser).To(Equal(int64(0)))
		})
	})

	Context("image configuration", func() {
		It("should set the image", func() {
			opts := baseOpts()
			opts.Image = "custom-image:v1.0"
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(spec.Containers[0].Image).To(Equal("custom-image:v1.0"))
		})

		It("should set the pull policy", func() {
			opts := baseOpts()
			opts.PullPolicy = "Always"
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullAlways))
		})

		It("should set image pull secret when specified", func() {
			opts := baseOpts()
			opts.ImagePullSecret = "my-registry-secret"
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(spec.ImagePullSecrets).To(HaveLen(1))
			Expect(spec.ImagePullSecrets[0].Name).To(Equal("my-registry-secret"))
		})

		It("should not set image pull secrets when empty", func() {
			opts := baseOpts()
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(spec.ImagePullSecrets).To(BeNil())
		})
	})

	Context("tolerations", func() {
		It("should parse single toleration", func() {
			opts := baseOpts()
			opts.Tolerations = []string{"dedicated=gpu:NoSchedule:Equal"}
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(spec.Tolerations).To(HaveLen(1))
			Expect(spec.Tolerations[0].Key).To(Equal("dedicated"))
			Expect(spec.Tolerations[0].Value).To(Equal("gpu"))
			Expect(spec.Tolerations[0].Effect).To(Equal(corev1.TaintEffectNoSchedule))
			Expect(spec.Tolerations[0].Operator).To(Equal(corev1.TolerationOpEqual))
		})

		It("should parse multiple tolerations", func() {
			opts := baseOpts()
			opts.Tolerations = []string{
				"::Exists",
				"node.kubernetes.io/not-ready=:NoExecute:Exists",
			}
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(spec.Tolerations).To(HaveLen(2))
		})

		It("should reject duplicate tolerations", func() {
			opts := baseOpts()
			opts.Tolerations = []string{
				"::Exists",
				"::Exists",
			}
			_, err := buildPodSpec(opts, false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("duplicate toleration"))
		})

		It("should reject invalid toleration format", func() {
			opts := baseOpts()
			opts.Tolerations = []string{"invalid"}
			_, err := buildPodSpec(opts, false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid toleration"))
		})

		It("should have no tolerations when none specified", func() {
			opts := baseOpts()
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(spec.Tolerations).To(BeNil())
		})
	})

	Context("node selector", func() {
		It("should set node selectors", func() {
			opts := baseOpts()
			opts.NodeSelector = map[string]string{
				"disktype":    "ssd",
				"environment": "production",
			}
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(spec.NodeSelector).To(HaveKeyWithValue("disktype", "ssd"))
			Expect(spec.NodeSelector).To(HaveKeyWithValue("environment", "production"))
		})

		It("should handle node selector with empty value", func() {
			opts := baseOpts()
			opts.NodeSelector = map[string]string{
				"node-role.kubernetes.io/control-plane": "",
			}
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(spec.NodeSelector).To(HaveKeyWithValue("node-role.kubernetes.io/control-plane", ""))
		})

		It("should not set node selector when empty", func() {
			opts := baseOpts()
			opts.NodeSelector = map[string]string{}
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(spec.NodeSelector).To(BeNil())
		})
	})

	Context("host network", func() {
		It("should enable host network when specified", func() {
			opts := baseOpts()
			opts.HostNetwork = true
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(spec.HostNetwork).To(BeTrue())
		})

		It("should not enable host network by default", func() {
			opts := baseOpts()
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(spec.HostNetwork).To(BeFalse())
		})
	})

	Context("volumes", func() {
		It("should add volumes and volume mounts from --volume flag", func() {
			opts := baseOpts()
			opts.Volumes = []string{"pvc:my-claim:/data", "secret:my-secret:/etc/creds:ro"}
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(spec.Volumes).To(HaveLen(2))
			Expect(spec.Volumes[0].PersistentVolumeClaim).NotTo(BeNil())
			Expect(spec.Volumes[0].PersistentVolumeClaim.ClaimName).To(Equal("my-claim"))
			Expect(spec.Volumes[1].Secret).NotTo(BeNil())
			Expect(spec.Volumes[1].Secret.SecretName).To(Equal("my-secret"))

			Expect(spec.Containers[0].VolumeMounts).To(HaveLen(2))
			Expect(spec.Containers[0].VolumeMounts[0].MountPath).To(Equal("/data"))
			Expect(spec.Containers[0].VolumeMounts[0].ReadOnly).To(BeFalse())
			Expect(spec.Containers[0].VolumeMounts[1].MountPath).To(Equal("/etc/creds"))
			Expect(spec.Containers[0].VolumeMounts[1].ReadOnly).To(BeTrue())
		})

		It("should have no volumes when none specified", func() {
			opts := baseOpts()
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(spec.Volumes).To(BeNil())
			Expect(spec.Containers[0].VolumeMounts).To(BeNil())
		})

		It("should return error for invalid volume format", func() {
			opts := baseOpts()
			opts.Volumes = []string{"invalid"}
			_, err := buildPodSpec(opts, false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid volume"))
		})

		It("should return error for unsupported volume type", func() {
			opts := baseOpts()
			opts.Volumes = []string{"nfs:server:/data"}
			_, err := buildPodSpec(opts, false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unsupported volume type"))
		})
	})

	Context("service account", func() {
		It("should use custom service account when specified", func() {
			opts := baseOpts()
			opts.ServiceAccount = "my-custom-sa"
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(spec.ServiceAccountName).To(Equal("my-custom-sa"))
		})

		It("should default to Namespace when service account is empty", func() {
			opts := baseOpts()
			opts.ServiceAccount = ""
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(spec.ServiceAccountName).To(Equal(Namespace))
		})
	})

	Context("pod defaults", func() {
		It("should set restart policy to Never", func() {
			opts := baseOpts()
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(spec.RestartPolicy).To(Equal(corev1.RestartPolicyNever))
		})

		It("should set service account name", func() {
			opts := baseOpts()
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(spec.ServiceAccountName).To(Equal(Namespace))
		})

		It("should enable automount service account token", func() {
			opts := baseOpts()
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(spec.AutomountServiceAccountToken).NotTo(BeNil())
			Expect(*spec.AutomountServiceAccountToken).To(BeTrue())
		})

		It("should set termination grace period to 300s", func() {
			opts := baseOpts()
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(spec.TerminationGracePeriodSeconds).NotTo(BeNil())
			Expect(*spec.TerminationGracePeriodSeconds).To(Equal(int64(300)))
		})

		It("should have a startup probe", func() {
			opts := baseOpts()
			spec, err := buildPodSpec(opts, false)
			Expect(err).NotTo(HaveOccurred())

			probe := spec.Containers[0].StartupProbe
			Expect(probe).NotTo(BeNil())
			Expect(probe.Exec.Command).To(ContainElement(ContainSubstring("/tmp/ready")))
		})
	})
})

var _ = Describe("buildDaemonPodSpec", func() {
	var baseOpts func() cmdoptions.ExecOptions

	BeforeEach(func() {
		baseOpts = func() cmdoptions.ExecOptions {
			return cmdoptions.ExecOptions{
				Image:         "docker.io/noksa/kubectl-helm:latest",
				PullPolicy:    "IfNotPresent",
				CpuRequest:    "500m",
				CpuLimit:      "1000m",
				MemoryRequest: "256Mi",
				MemoryLimit:   "512Mi",
				RunAsUser:     -1,
				RunAsGroup:    -1,
				Timeout:       10 * time.Minute,
			}
		}
	})

	It("should override command for daemon mode", func() {
		opts := baseOpts()
		spec, err := buildDaemonPodSpec(opts)
		Expect(err).NotTo(HaveOccurred())

		Expect(spec.Containers[0].Command).To(Equal([]string{"sh", "-c"}))
		Expect(spec.Containers[0].Args[0]).To(ContainSubstring("sleep infinity"))
		Expect(spec.Containers[0].Args[0]).To(ContainSubstring("touch /tmp/ready"))
	})

	It("should clear env vars for daemon pods", func() {
		opts := baseOpts()
		opts.Env = map[string]string{"FOO": "bar"}
		spec, err := buildDaemonPodSpec(opts)
		Expect(err).NotTo(HaveOccurred())

		Expect(spec.Containers[0].Env).To(BeNil())
	})

	It("should preserve resource settings from exec options", func() {
		opts := baseOpts()
		spec, err := buildDaemonPodSpec(opts)
		Expect(err).NotTo(HaveOccurred())

		res := spec.Containers[0].Resources
		Expect(res.Requests.Cpu().String()).To(Equal("500m"))
	})

	It("should preserve tolerations from exec options", func() {
		opts := baseOpts()
		opts.Tolerations = []string{"::Exists"}
		spec, err := buildDaemonPodSpec(opts)
		Expect(err).NotTo(HaveOccurred())

		Expect(spec.Tolerations).To(HaveLen(1))
	})

	It("should preserve security context from exec options", func() {
		opts := baseOpts()
		opts.RunAsUser = 1000
		spec, err := buildDaemonPodSpec(opts)
		Expect(err).NotTo(HaveOccurred())

		Expect(spec.Containers[0].SecurityContext.RunAsUser).NotTo(BeNil())
		Expect(*spec.Containers[0].SecurityContext.RunAsUser).To(Equal(int64(1000)))
	})

	It("should propagate buildPodSpec errors", func() {
		opts := baseOpts()
		opts.Tolerations = []string{"invalid"}
		_, err := buildDaemonPodSpec(opts)
		Expect(err).To(HaveOccurred())
	})

	It("should preserve volumes from exec options", func() {
		opts := baseOpts()
		opts.Volumes = []string{"pvc:my-claim:/data"}
		spec, err := buildDaemonPodSpec(opts)
		Expect(err).NotTo(HaveOccurred())

		Expect(spec.Volumes).To(HaveLen(1))
		Expect(spec.Containers[0].VolumeMounts).To(HaveLen(1))
	})

	It("should preserve service account from exec options", func() {
		opts := baseOpts()
		opts.ServiceAccount = "custom-sa"
		spec, err := buildDaemonPodSpec(opts)
		Expect(err).NotTo(HaveOccurred())

		Expect(spec.ServiceAccountName).To(Equal("custom-sa"))
	})
})

// --- helpers ---

func envVarNames(envs []corev1.EnvVar) []string {
	names := make([]string, len(envs))
	for i, e := range envs {
		names[i] = e.Name
	}
	return names
}

func findEnvVar(envs []corev1.EnvVar, name string) string {
	for _, e := range envs {
		if e.Name == name {
			return e.Value
		}
	}
	return ""
}
