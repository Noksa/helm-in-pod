package hipembedded

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestHipembedded(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Hipembedded Suite")
}
