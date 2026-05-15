package cmdoptions_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/noksa/helm-in-pod/internal/cmdoptions"
)

var _ = Describe("MaskSetValues", func() {
	DescribeTable("masks --set values correctly",
		func(input, expected string) {
			Expect(cmdoptions.MaskSetValues(input)).To(Equal(expected))
		},
		Entry("space-separated --set",
			"helm upgrade --set config.token=secret123 myapp repo/chart",
			"helm upgrade --set config.token=*** myapp repo/chart"),
		Entry("equals-joined --set=",
			"helm upgrade --set=config.token=secret123 myapp repo/chart",
			"helm upgrade --set=config.token=*** myapp repo/chart"),
		Entry("multiple comma-separated values",
			"helm upgrade --set key1=val1,key2=val2 myapp",
			"helm upgrade --set key1=***,key2=*** myapp"),
		Entry("--set-string",
			"helm upgrade --set-string password=s3cret myapp",
			"helm upgrade --set-string password=*** myapp"),
		Entry("--set-file",
			"helm upgrade --set-file ca=/tmp/ca.pem myapp",
			"helm upgrade --set-file ca=*** myapp"),
		Entry("--set-json",
			`helm upgrade --set-json config={"key":"val"} myapp`,
			`helm upgrade --set-json config=*** myapp`),
		Entry("no --set flags leaves command unchanged",
			"helm list -A",
			"helm list -A"),
		Entry("multiple --set flags",
			"helm upgrade --set a=1 --set b=2 myapp",
			"helm upgrade --set a=*** --set b=*** myapp"),
		Entry("mixed flags",
			"helm diff upgrade --install -n ns myapp /tmp/chart --version 1.0 -f /tmp/values.yaml --set config.bot.slack.botToken=xoxb-5442749",
			"helm diff upgrade --install -n ns myapp /tmp/chart --version 1.0 -f /tmp/values.yaml --set config.bot.slack.botToken=***"),
		Entry("value with equals sign",
			"helm upgrade --set connStr=postgres://user:pass@host/db?opt=val myapp",
			"helm upgrade --set connStr=*** myapp"),
	)
})
