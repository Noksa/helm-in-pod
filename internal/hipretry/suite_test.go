package hipretry

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestHipretry(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Hipretry Suite")
}
