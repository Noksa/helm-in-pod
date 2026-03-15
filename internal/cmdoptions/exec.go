package cmdoptions

import "time"

type ExecOptions struct {
	Image           string
	Files           []string
	CopyRepo        bool
	Labels          map[string]string
	Annotations     map[string]string
	UpdateRepo      []string
	UpdateAllRepos  bool
	Cpu             string // Deprecated: use CpuRequest instead
	Memory          string // Deprecated: use MemoryRequest instead
	CpuRequest      string
	CpuLimit        string
	MemoryRequest   string
	MemoryLimit     string
	Env             map[string]string
	FilesAsMap      map[string]string
	SubstEnv        []string
	RunAsUser       int64
	Tolerations     []string
	NodeSelector    map[string]string
	RunAsGroup      int64
	ImagePullSecret string
	PullPolicy      string
	HostNetwork     bool
	CreatePDB       bool
	// Timeout is duration from --timeout flag + 10 minutes
	// set internally
	Timeout            time.Duration
	CopyAttempts       int
	UpdateRepoAttempts int
	Volumes            []string
	ServiceAccount     string
	DryRun             bool
	CopyFrom           []string
}
