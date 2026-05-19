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

	DescribeTable("narrow edge cases — regression guards",
		func(input, expected string) {
			Expect(cmdoptions.MaskSetValues(input)).To(Equal(expected))
		},
		Entry("double-quoted value with spaces is masked once",
			`helm upgrade --set "key=value with spaces" myapp`,
			`helm upgrade --set "key=*** myapp`),
		Entry("single-quoted value with spaces is masked once",
			`helm upgrade --set 'key=value with spaces' myapp`,
			`helm upgrade --set 'key=*** myapp`),
		Entry("shell-escaped quotes around value are masked",
			`helm upgrade --set key=\"value\" myapp`,
			`helm upgrade --set key=*** myapp`),
		Entry("unclosed single quote consumes rest of command but still masks value",
			`helm upgrade --set 'key=unfinished myapp`,
			`helm upgrade --set 'key=***`),
		Entry("value with multiple equals signs only masks after first",
			"helm upgrade --set token=ey=encoded= myapp",
			"helm upgrade --set token=*** myapp"),
		Entry("empty value is still masked",
			"helm upgrade --set key= myapp",
			"helm upgrade --set key=*** myapp"),
		Entry("--set without trailing arg does not crash",
			"helm upgrade --set",
			"helm upgrade --set"),
		Entry("--set with no = in next arg is left unchanged (no value to mask)",
			"helm upgrade --set keyonly myapp",
			"helm upgrade --set keyonly myapp"),
	)

	DescribeTable("KNOWN LIMITATIONS — comma inside quoted values leaks fragments",
		func(input, expected string) {
			Expect(cmdoptions.MaskSetValues(input)).To(Equal(expected),
				"comma-in-quoted-value leak: if this test fails after a fix, update the expected string")
		},
		Entry("double-quoted value containing commas leaks fragments after first comma",
			`helm upgrade --set list="a,b,c" myapp`,
			`helm upgrade --set list=***,b,c" myapp`),
		Entry("--set= equals-joined with comma in quoted value leaks fragments",
			`helm upgrade --set=key="val,with,commas" myapp`,
			`helm upgrade --set=key=***,with,commas" myapp`),
		Entry("backslash-escaped comma is not respected, leaks fragment",
			`helm upgrade --set key=a\,b,key2=c myapp`,
			`helm upgrade --set key=***,b,key2=*** myapp`),
	)
})
