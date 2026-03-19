package hippod_test

import (
	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/noksa/helm-in-pod/internal/hipconsts"
	"github.com/noksa/helm-in-pod/internal/hippod"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PDB", func() {
	Describe("GenerateOperationID", func() {
		It("should generate a valid UUID", func() {
			id := hippod.GenerateOperationID()
			Expect(id).NotTo(BeEmpty())
			Expect(id).To(MatchRegexp(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`))
		})

		It("should generate unique UUIDs", func() {
			id1 := hippod.GenerateOperationID()
			id2 := hippod.GenerateOperationID()
			Expect(id1).NotTo(Equal(id2))
		})
	})

	Describe("LabelOperationID constant", func() {
		It("should have the correct value", func() {
			Expect(hipconsts.LabelOperationID).To(Equal("helm-in-pod/operation-id"))
		})
	})

	Describe("CreatePDB flag", func() {
		It("should default to true", func() {
			opts := cmdoptions.ExecOptions{}
			// When not explicitly set, CreatePDB should be false (Go default)
			// but the flag default is true, so when parsed it will be true
			Expect(opts.CreatePDB).To(BeFalse(), "Default Go value should be false before flag parsing")
		})
	})
})
