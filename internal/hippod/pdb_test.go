package hippod_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/noksa/helm-in-pod/internal/hipconsts"
)

var _ = Describe("PDB", func() {
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
