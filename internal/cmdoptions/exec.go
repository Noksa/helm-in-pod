package cmdoptions

import "time"

type ExecOptions struct {
	Image      string
	Files      []string
	CopyRepo   bool
	Labels     map[string]string
	UpdateRepo []string
	Cpu        string
	Memory     string
	Env        map[string]string
	FilesAsMap map[string]string
	Timeout    time.Duration
	SubstEnv   []string
}
