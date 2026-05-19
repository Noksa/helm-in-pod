package internal

import (
	"context"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

var _ = Describe("RunCommand", func() {
	var savedArgs []string

	BeforeEach(func() {
		savedArgs = os.Args
	})
	AfterEach(func() {
		os.Args = savedArgs
	})

	newProbe := func() (*cobra.Command, *time.Duration, *error) {
		var capturedDeadline time.Duration
		var capturedErr error
		cmd := &cobra.Command{
			Use:           "probe",
			SilenceErrors: true,
			SilenceUsage:  true,
			RunE: func(c *cobra.Command, _ []string) error {
				ctx := c.Context()
				deadline, ok := ctx.Deadline()
				if ok {
					capturedDeadline = time.Until(deadline)
				}
				capturedErr = ctx.Err()
				return nil
			},
		}
		cmd.PersistentFlags().Duration("timeout", 0, "")
		return cmd, &capturedDeadline, &capturedErr
	}

	It("applies the 2h default when --timeout is not provided", func() {
		os.Args = []string{"prog"}
		cmd, deadline, ctxErr := newProbe()
		Expect(RunCommand(cmd)).To(Succeed())
		Expect(*ctxErr).ToNot(HaveOccurred())
		Expect(*deadline).To(BeNumerically(">", 1*time.Hour+50*time.Minute))
		Expect(*deadline).To(BeNumerically("<=", 2*time.Hour))
	})

	It("applies the 2h default when --timeout=0", func() {
		os.Args = []string{"prog", "--timeout=0"}
		cmd, deadline, _ := newProbe()
		Expect(RunCommand(cmd)).To(Succeed())
		Expect(*deadline).To(BeNumerically(">", 1*time.Hour+50*time.Minute))
		Expect(*deadline).To(BeNumerically("<=", 2*time.Hour))
	})

	It("applies the 2h default when --timeout is negative", func() {
		os.Args = []string{"prog", "--timeout=-30m"}
		cmd, deadline, _ := newProbe()
		Expect(RunCommand(cmd)).To(Succeed())
		Expect(*deadline).To(BeNumerically(">", 1*time.Hour+50*time.Minute))
		Expect(*deadline).To(BeNumerically("<=", 2*time.Hour))
	})

	It("honors an explicit --timeout=30m", func() {
		os.Args = []string{"prog", "--timeout=30m"}
		cmd, deadline, _ := newProbe()
		Expect(RunCommand(cmd)).To(Succeed())
		Expect(*deadline).To(BeNumerically(">", 29*time.Minute))
		Expect(*deadline).To(BeNumerically("<=", 30*time.Minute))
	})

	It("honors a very short explicit --timeout", func() {
		os.Args = []string{"prog", "--timeout=2s"}
		cmd, deadline, _ := newProbe()
		Expect(RunCommand(cmd)).To(Succeed())
		Expect(*deadline).To(BeNumerically(">", 1500*time.Millisecond))
		Expect(*deadline).To(BeNumerically("<=", 2*time.Second))
	})

	It("pre-parses --timeout even when subcommand flags come first in os.Args", func() {
		os.Args = []string{"prog", "--timeout=15m"}
		cmd, deadline, _ := newProbe()
		Expect(RunCommand(cmd)).To(Succeed())
		Expect(*deadline).To(BeNumerically(">", 14*time.Minute))
		Expect(*deadline).To(BeNumerically("<=", 15*time.Minute))
	})

	It("propagates context cancellation to the RunE handler", func() {
		os.Args = []string{"prog", "--timeout=10ms"}
		var observed error
		cmd := &cobra.Command{
			Use:           "probe",
			SilenceErrors: true,
			SilenceUsage:  true,
			RunE: func(c *cobra.Command, _ []string) error {
				select {
				case <-c.Context().Done():
					observed = c.Context().Err()
					return nil
				case <-time.After(500 * time.Millisecond):
					return nil
				}
			},
		}
		cmd.PersistentFlags().Duration("timeout", 0, "")
		Expect(RunCommand(cmd)).To(Succeed())
		Expect(observed).To(Equal(context.DeadlineExceeded))
	})
})
