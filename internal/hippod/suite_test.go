package hippod

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestHippod(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Hippod Suite")
}
