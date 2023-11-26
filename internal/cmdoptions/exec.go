package cmdoptions

import "time"

type ExecOptions struct {
	Image           string
	Files           []string
	CopyRepo        bool
	Labels          map[string]string
	Annotations     map[string]string
	UpdateRepo      []string
	Cpu             string
	Memory          string
	Env             map[string]string
	FilesAsMap      map[string]string
	SubstEnv        []string
	RunAsUser       int64
	RunAsGroup      int64
	ImagePullSecret string
	PullPolicy      string
	// Timeout is duration from --timeout flag + 10 minutes
	// set internally
	Timeout            time.Duration
	CopyAttempts       int
	UpdateRepoAttempts int
}
