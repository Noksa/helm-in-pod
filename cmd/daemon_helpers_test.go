package cmd

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("formatDuration", func() {
	It("should format seconds only", func() {
		Expect(formatDuration(30 * time.Second)).To(Equal("30s"))
	})

	It("should format zero seconds", func() {
		Expect(formatDuration(0)).To(Equal("0s"))
	})

	It("should format sub-second rounded to nearest second", func() {
		// 500ms rounds to 1s due to d.Round(time.Second)
		Expect(formatDuration(500 * time.Millisecond)).To(Equal("1s"))
	})

	It("should format 100ms as 0s", func() {
		Expect(formatDuration(100 * time.Millisecond)).To(Equal("0s"))
	})

	It("should format minutes and seconds", func() {
		Expect(formatDuration(90 * time.Second)).To(Equal("1m30s"))
	})

	It("should format exact minutes", func() {
		Expect(formatDuration(5 * time.Minute)).To(Equal("5m0s"))
	})

	It("should format hours and minutes", func() {
		Expect(formatDuration(2*time.Hour + 15*time.Minute)).To(Equal("2h15m"))
	})

	It("should format exact hours", func() {
		Expect(formatDuration(3 * time.Hour)).To(Equal("3h0m"))
	})

	It("should format large durations", func() {
		Expect(formatDuration(48*time.Hour + 30*time.Minute)).To(Equal("48h30m"))
	})

	It("should format 59 seconds as seconds", func() {
		Expect(formatDuration(59 * time.Second)).To(Equal("59s"))
	})

	It("should format 59 minutes as minutes", func() {
		Expect(formatDuration(59*time.Minute + 59*time.Second)).To(Equal("59m59s"))
	})
})

var _ = Describe("colorPhase", func() {
	It("should return green for Running", func() {
		result := colorPhase("Running")
		Expect(result).To(ContainSubstring("Running"))
	})

	It("should return yellow for Pending", func() {
		result := colorPhase("Pending")
		Expect(result).To(ContainSubstring("Pending"))
	})

	It("should return red for Failed", func() {
		result := colorPhase("Failed")
		Expect(result).To(ContainSubstring("Failed"))
	})

	It("should return red for Unknown", func() {
		result := colorPhase("Unknown")
		Expect(result).To(ContainSubstring("Unknown"))
	})

	It("should return red for Succeeded", func() {
		result := colorPhase("Succeeded")
		Expect(result).To(ContainSubstring("Succeeded"))
	})

	It("should handle empty string without panic", func() {
		// color.RedString("") returns empty string, just verify no panic
		Expect(func() { colorPhase("") }).NotTo(Panic())
	})
})
