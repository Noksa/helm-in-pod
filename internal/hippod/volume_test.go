package hippod

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("parseVolume", func() {
	Context("PVC volumes", func() {
		It("should parse a basic PVC volume", func() {
			vol, mount, err := parseVolume("pvc:my-claim:/data")
			Expect(err).NotTo(HaveOccurred())
			Expect(vol.Name).To(Equal("my-claim"))
			Expect(vol.PersistentVolumeClaim).NotTo(BeNil())
			Expect(vol.PersistentVolumeClaim.ClaimName).To(Equal("my-claim"))
			Expect(vol.PersistentVolumeClaim.ReadOnly).To(BeFalse())
			Expect(mount.Name).To(Equal("my-claim"))
			Expect(mount.MountPath).To(Equal("/data"))
			Expect(mount.ReadOnly).To(BeFalse())
		})

		It("should parse a read-only PVC volume", func() {
			vol, mount, err := parseVolume("pvc:my-claim:/data:ro")
			Expect(err).NotTo(HaveOccurred())
			Expect(vol.PersistentVolumeClaim.ReadOnly).To(BeTrue())
			Expect(mount.ReadOnly).To(BeTrue())
		})
	})

	Context("Secret volumes", func() {
		It("should parse a basic secret volume", func() {
			vol, mount, err := parseVolume("secret:my-secret:/etc/creds")
			Expect(err).NotTo(HaveOccurred())
			Expect(vol.Name).To(Equal("my-secret"))
			Expect(vol.Secret).NotTo(BeNil())
			Expect(vol.Secret.SecretName).To(Equal("my-secret"))
			Expect(mount.MountPath).To(Equal("/etc/creds"))
			Expect(mount.ReadOnly).To(BeFalse())
		})

		It("should parse a read-only secret volume", func() {
			_, mount, err := parseVolume("secret:my-secret:/etc/creds:ro")
			Expect(err).NotTo(HaveOccurred())
			Expect(mount.ReadOnly).To(BeTrue())
		})
	})

	Context("ConfigMap volumes", func() {
		It("should parse a basic configmap volume", func() {
			vol, mount, err := parseVolume("configmap:my-cm:/etc/config")
			Expect(err).NotTo(HaveOccurred())
			Expect(vol.Name).To(Equal("my-cm"))
			Expect(vol.ConfigMap).NotTo(BeNil())
			Expect(vol.ConfigMap.Name).To(Equal("my-cm"))
			Expect(mount.MountPath).To(Equal("/etc/config"))
		})

		It("should parse a read-only configmap volume", func() {
			_, mount, err := parseVolume("configmap:my-cm:/etc/config:ro")
			Expect(err).NotTo(HaveOccurred())
			Expect(mount.ReadOnly).To(BeTrue())
		})
	})

	Context("HostPath volumes", func() {
		It("should parse a basic hostpath volume", func() {
			vol, mount, err := parseVolume("hostpath:/var/log:/host-logs")
			Expect(err).NotTo(HaveOccurred())
			Expect(vol.HostPath).NotTo(BeNil())
			Expect(vol.HostPath.Path).To(Equal("/var/log"))
			Expect(mount.MountPath).To(Equal("/host-logs"))
		})

		It("should parse a read-only hostpath volume", func() {
			_, mount, err := parseVolume("hostpath:/var/log:/host-logs:ro")
			Expect(err).NotTo(HaveOccurred())
			Expect(mount.ReadOnly).To(BeTrue())
		})
	})

	Context("name sanitization", func() {
		It("should replace slashes with dashes in volume name", func() {
			vol, mount, err := parseVolume("hostpath:/var/log/app:/mnt")
			Expect(err).NotTo(HaveOccurred())
			Expect(vol.Name).To(Equal("var-log-app"))
			Expect(mount.Name).To(Equal("var-log-app"))
		})

		It("should replace underscores with dashes", func() {
			vol, _, err := parseVolume("pvc:my_claim_name:/data")
			Expect(err).NotTo(HaveOccurred())
			Expect(vol.Name).To(Equal("my-claim-name"))
		})

		It("should trim leading dashes", func() {
			vol, _, err := parseVolume("hostpath:/some/path:/mnt")
			Expect(err).NotTo(HaveOccurred())
			Expect(vol.Name).NotTo(HavePrefix("-"))
		})

		It("should truncate names longer than 63 characters", func() {
			longName := "a-very-long-persistent-volume-claim-name-that-exceeds-the-sixty-three-character-limit"
			vol, _, err := parseVolume("pvc:" + longName + ":/data")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(vol.Name)).To(BeNumerically("<=", 63))
		})
	})

	Context("case insensitivity", func() {
		It("should accept uppercase volume type", func() {
			vol, _, err := parseVolume("PVC:my-claim:/data")
			Expect(err).NotTo(HaveOccurred())
			Expect(vol.PersistentVolumeClaim).NotTo(BeNil())
		})

		It("should accept mixed case volume type", func() {
			vol, _, err := parseVolume("Secret:my-secret:/etc/creds")
			Expect(err).NotTo(HaveOccurred())
			Expect(vol.Secret).NotTo(BeNil())
		})
	})

	Context("error cases", func() {
		It("should reject too few parts", func() {
			_, _, err := parseVolume("pvc:my-claim")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected format"))
		})

		It("should reject single value", func() {
			_, _, err := parseVolume("invalid")
			Expect(err).To(HaveOccurred())
		})

		It("should reject unsupported volume type", func() {
			_, _, err := parseVolume("nfs:server:/data")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unsupported volume type"))
			Expect(err.Error()).To(ContainSubstring("nfs"))
		})

		It("should reject empty string", func() {
			_, _, err := parseVolume("")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("non-ro fourth part", func() {
		It("should not set readOnly when fourth part is not 'ro'", func() {
			_, mount, err := parseVolume("pvc:my-claim:/data:rw")
			Expect(err).NotTo(HaveOccurred())
			Expect(mount.ReadOnly).To(BeFalse())
		})
	})
})
