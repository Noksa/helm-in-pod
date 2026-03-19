package helmtar

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestHelmtar(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Helmtar Suite")
}
