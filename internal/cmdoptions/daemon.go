package cmdoptions

type DaemonOptions struct {
	ExecOptions
	Name  string
	Force bool
	Clean []string
}
