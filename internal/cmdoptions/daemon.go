package cmdoptions

type DaemonOptions struct {
	ExecOptions
	Name  string
	Clean []string
}
