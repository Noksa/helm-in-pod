package internal

type HelmInPodFlags struct {
	Image      string
	Files      string
	CopyRepo   bool
	UpdateRepo []string
	Env        map[string]string
	FilesAsMap map[string]string
}
