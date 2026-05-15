package cmdoptions_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCmdoptions(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cmdoptions Suite")
}
